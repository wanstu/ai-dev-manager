package cli

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/gateway"
	"ai-dev-manager-v2/internal/gatewayservice"
	"ai-dev-manager-v2/internal/model"
)

type gatewayInstallPlan struct {
	Action             string   `json:"action"`
	User               string   `json:"user"`
	Listen             string   `json:"listen"`
	StatePath          string   `json:"state_path"`
	Executable         string   `json:"executable"`
	AllowedHosts       []string `json:"allowed_hosts"`
	AdminKeyAction     string   `json:"admin_key_action"`
	AgentKeyAction     string   `json:"agent_key_action"`
	ServiceEnabled     bool     `json:"service_enabled"`
	CurrentUnitVersion int      `json:"current_unit_version,omitempty"`
	TargetUnitVersion  int      `json:"target_unit_version"`
	Changes            []string `json:"changes"`
}

func runGatewayService(args []string) error {
	if wantsHelp(args) {
		fmt.Fprintln(os.Stdout, `用法：
  adm gateway service install [--user USER] [--listen HOST:PORT]
  adm gateway service status
  sudo adm gateway service start
  sudo adm gateway service stop
  sudo adm gateway service restart
  sudo adm gateway service enable
  sudo adm gateway service disable
  sudo adm gateway service uninstall

当前第一版在 Linux 上使用 systemd；CLI 名称保持跨平台，不暴露 systemd 作为产品概念。`)
		return nil
	}
	if !gatewayservice.Supported() {
		return fmt.Errorf("gateway service management is not implemented on this platform yet")
	}
	switch args[0] {
	case "install":
		fs := newFlagSet("gateway service install", func() {
			fmt.Fprintln(os.Stdout, "用法：sudo adm gateway service install [--user USER] [--listen HOST:PORT]")
			fmt.Fprintln(os.Stdout, "首次安装需要 --user；已由 ADM 管理的 service 可重复执行并沿用原 user/listen/Enabled 状态。")
		})
		userName := fs.String("user", "", "运行 Gateway 的系统用户")
		listen := fs.String("listen", "", "Gateway 监听地址；首次安装默认 0.0.0.0:8001，升级时省略则保留原监听地址")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("gateway service install 只接受 --user / --listen")
		}
		if err := gatewayservice.CheckManagementPrivileges(); err != nil {
			return err
		}
		previous, err := gatewayservice.Inspect()
		if err != nil {
			return err
		}
		if previous.Installed && !previous.Managed {
			return fmt.Errorf("%s 已存在但不是 ADM 管理的 Gateway service；拒绝覆盖", previous.UnitPath)
		}
		targetUserName, targetListen, err := resolveGatewayInstallTarget(previous, *userName, *listen, 0)
		if err != nil {
			return err
		}
		target, err := gatewayservice.ResolveTargetUser(targetUserName)
		if err != nil {
			return err
		}
		if previous.Installed && strings.EqualFold(strings.TrimSpace(previous.User), strings.TrimSpace(target.Name)) && strings.TrimSpace(previous.StatePath) != "" {
			target.StatePath = previous.StatePath
		}
		targetService := app.New(target.StatePath)
		if err := validateGatewayStartReadiness(targetService, targetListen); err != nil {
			return fmt.Errorf("目标用户 %s 的 Gateway 尚未完成远程配置：%w\n建议直接运行：sudo adm gateway install --user %s --remote --listen %s", target.Name, err, target.Name, targetListen)
		}
		executable, err := currentExecutablePath()
		if err != nil {
			return err
		}
		if err := gatewayservice.FixStateOwnership(target); err != nil {
			return fmt.Errorf("修正 %s ADM 状态目录 owner: %w", target.Name, err)
		}
		options := gatewayservice.InstallOptions{
			User: target, Listen: targetListen, Executable: executable, Start: true,
		}
		status, created, err := applyGatewayService(previous, options)
		if err != nil {
			return err
		}
		if _, err := gateway.WaitHTTPReady(targetListen, 10*time.Second); err != nil {
			if created {
				_ = gatewayservice.Uninstall()
			} else {
				_, _ = gatewayservice.Stop()
				_ = restoreGatewayService(previous)
			}
			return fmt.Errorf("Gateway service 已启动但未 ready，已尝试恢复之前的 service 配置：%w", err)
		}
		if created {
			fmt.Println("ADM Gateway service 已安装")
		} else {
			fmt.Println("ADM Gateway service 配置已更新")
		}
		printGatewayServiceStatus(status)
		return nil
	case "status":
		if len(args) != 1 {
			return fmt.Errorf("gateway service status 不接受额外参数")
		}
		status, err := gatewayservice.Inspect()
		if err != nil {
			return err
		}
		printGatewayServiceStatus(status)
		if status.Active && status.Listen != "" {
			gatewayStatus, inspectErr := gateway.InspectHTTP(status.Listen)
			if inspectErr == nil && gatewayStatus.State == gateway.HTTPStateRunning {
				fmt.Println("Gateway health： ready")
				fmt.Println("Gateway PID：   ", gatewayStatus.PID)
				printGatewayIdentity(gatewayStatus.PID)
			} else if inspectErr != nil {
				fmt.Println("Gateway health： error -", inspectErr)
			} else {
				fmt.Println("Gateway health：", gatewayStatus.State)
			}
		}
		return nil
	case "start":
		if len(args) != 1 {
			return fmt.Errorf("gateway service start 不接受额外参数")
		}
		if err := gatewayservice.CheckManagementPrivileges(); err != nil {
			return err
		}
		current, err := gatewayservice.Inspect()
		if err != nil {
			return err
		}
		if err := validateGatewayServiceStartReadiness(current); err != nil {
			return err
		}
		status, err := gatewayservice.Start()
		if err != nil {
			return err
		}
		if err := waitGatewayServiceReady(status); err != nil {
			return fmt.Errorf("systemd service 已启动，但 Gateway 未 ready：%w", err)
		}
		printGatewayServiceStatus(status)
		return nil
	case "stop":
		if len(args) != 1 {
			return fmt.Errorf("gateway service stop 不接受额外参数")
		}
		if err := gatewayservice.CheckManagementPrivileges(); err != nil {
			return err
		}
		status, err := gatewayservice.Stop()
		if err != nil {
			return err
		}
		printGatewayServiceStatus(status)
		return nil
	case "restart":
		if len(args) != 1 {
			return fmt.Errorf("gateway service restart 不接受额外参数")
		}
		if err := gatewayservice.CheckManagementPrivileges(); err != nil {
			return err
		}
		current, err := gatewayservice.Inspect()
		if err != nil {
			return err
		}
		if err := validateGatewayServiceStartReadiness(current); err != nil {
			return err
		}
		status, err := gatewayservice.Restart()
		if err != nil {
			return err
		}
		if err := waitGatewayServiceReady(status); err != nil {
			return fmt.Errorf("systemd service 已重启，但 Gateway 未 ready：%w", err)
		}
		printGatewayServiceStatus(status)
		return nil
	case "enable", "disable":
		if len(args) != 1 {
			return fmt.Errorf("gateway service %s 不接受额外参数", args[0])
		}
		if err := gatewayservice.CheckManagementPrivileges(); err != nil {
			return err
		}
		status, err := gatewayservice.SetEnabled(args[0] == "enable")
		if err != nil {
			return err
		}
		printGatewayServiceStatus(status)
		return nil
	case "uninstall":
		if len(args) != 1 {
			return fmt.Errorf("gateway service uninstall 不接受额外参数")
		}
		if err := gatewayservice.CheckManagementPrivileges(); err != nil {
			return err
		}
		if err := gatewayservice.Uninstall(); err != nil {
			return err
		}
		fmt.Println("ADM Gateway service 已卸载")
		return nil
	default:
		return fmt.Errorf("未知 gateway service 命令 %q；运行 adm gateway service -h 查看帮助", args[0])
	}
}

func runGatewayInstall(args []string) error {
	fs := newFlagSet("gateway install", func() {
		fmt.Fprintln(os.Stdout, "用法：sudo adm gateway install --remote [--user USER] [--port PORT | --listen HOST:PORT] [--hosts HOST1,HOST2] [--rotate-keys] [--dry-run]")
		fmt.Fprintln(os.Stdout, "\n首次安装需要 --user；已有 ADM managed service 可重复执行用于升级，省略 user/listen/hosts 时保留原配置，Enabled 状态也保持不变。")
	})
	userName := fs.String("user", "", "运行 Gateway 的系统用户；首次安装必填，升级时省略则沿用原用户")
	remote := fs.Bool("remote", false, "安装远程 Gateway")
	port := fs.Int("port", 0, "监听端口；首次安装未指定时默认 8001，升级时省略则保留原端口")
	listen := fs.String("listen", "", "完整监听地址；例如 0.0.0.0:8001")
	hostsRaw := fs.String("hosts", "*", "允许的 Host/IP，逗号分隔；首次安装默认 *，升级时省略则保留原白名单")
	rotateKeys := fs.Bool("rotate-keys", false, "即使已有 Key 也重新生成双 Key")
	dryRun := fs.Bool("dry-run", false, "只输出部署计划，不修改 Gateway state 或 system service")
	if err := fs.Parse(args); err != nil {
		return flagError(err)
	}
	if fs.NArg() != 0 || !*remote {
		return fmt.Errorf("gateway install 当前要求 --remote；首次安装还需要 --user USER")
	}
	if !gatewayservice.Supported() {
		return fmt.Errorf("gateway install 当前仅支持 Linux/systemd")
	}
	existing, err := gatewayservice.Inspect()
	if err != nil {
		return err
	}
	if existing.Installed && !existing.Managed {
		return fmt.Errorf("%s 已存在但不是 ADM 管理的 Gateway service；拒绝覆盖", existing.UnitPath)
	}

	targetUserName, targetListen, err := resolveGatewayInstallTarget(existing, *userName, *listen, *port)
	if err != nil {
		return err
	}

	if !*dryRun {
		if err := gatewayservice.CheckManagementPrivileges(); err != nil {
			return err
		}
	}
	target, err := gatewayservice.ResolveTargetUser(targetUserName)
	if err != nil {
		return err
	}
	if existing.Installed && strings.EqualFold(strings.TrimSpace(existing.User), strings.TrimSpace(target.Name)) && strings.TrimSpace(existing.StatePath) != "" {
		target.StatePath = existing.StatePath
	}
	executable, err := currentExecutablePath()
	if err != nil {
		return err
	}
	targetService := app.New(target.StatePath)
	stateDir := filepath.Dir(target.StatePath)
	_, stateDirStatErr := os.Stat(stateDir)
	stateDirExisted := stateDirStatErr == nil
	if stateDirStatErr != nil && !os.IsNotExist(stateDirStatErr) {
		return fmt.Errorf("检查目标用户 %s 的 ADM 状态目录: %w", target.Name, stateDirStatErr)
	}
	_, stateStatErr := os.Stat(target.StatePath)
	stateExisted := stateStatErr == nil
	if stateStatErr != nil && !os.IsNotExist(stateStatErr) {
		return fmt.Errorf("检查目标用户 %s 的 ADM 状态文件: %w", target.Name, stateStatErr)
	}
	previous, err := targetService.GatewayAccessConfig()
	if err != nil {
		return fmt.Errorf("读取目标用户 %s 的 Gateway access 配置: %w", target.Name, err)
	}
	hosts := resolveGatewayInstallHosts(previous.AllowedHosts, *hostsRaw, flagWasSet(fs, "hosts"))
	plan := buildGatewayInstallPlan(existing, target, targetListen, executable, previous, hosts, *rotateKeys)
	if *dryRun {
		return writeJSON(plan)
	}
	serviceApplied := false
	createdService := !existing.Installed
	restoreState := func() error {
		if stateExisted {
			if err := targetService.Store.Update(func(state *model.State) error {
				state.GatewayAccess = previous
				return nil
			}); err != nil {
				return err
			}
			return gatewayservice.FixStateOwnership(target)
		}
		if err := os.Remove(target.StatePath); err != nil && !os.IsNotExist(err) {
			return err
		}
		if !stateDirExisted {
			if err := os.Remove(stateDir); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		return nil
	}
	rollback := func(cause error) error {
		warnings := make([]string, 0)
		if serviceApplied {
			if createdService {
				if err := gatewayservice.Uninstall(); err != nil {
					warnings = append(warnings, "卸载新 service 失败: "+err.Error())
				}
			} else if _, err := gatewayservice.Stop(); err != nil {
				warnings = append(warnings, "停止新 service 失败: "+err.Error())
			}
		}
		if err := restoreState(); err != nil {
			warnings = append(warnings, "恢复 Gateway access 状态失败: "+err.Error())
		}
		if serviceApplied && !createdService {
			if err := restoreGatewayService(existing); err != nil {
				warnings = append(warnings, "恢复原 service 配置失败: "+err.Error())
			}
		}
		if len(warnings) > 0 {
			return fmt.Errorf("%w；回滚警告：%s", cause, strings.Join(warnings, "；"))
		}
		return cause
	}

	result, err := targetService.SetupGatewayRemote(hosts, *rotateKeys)
	if err != nil {
		return rollback(err)
	}
	if err := gatewayservice.FixStateOwnership(target); err != nil {
		return rollback(fmt.Errorf("修正 %s ADM 状态目录 owner: %w", target.Name, err))
	}
	status, created, err := applyGatewayService(existing, gatewayservice.InstallOptions{
		User: target, Listen: targetListen, Executable: executable, Start: true,
	})
	if err != nil {
		return rollback(err)
	}
	serviceApplied = true
	createdService = created
	if _, err := gateway.WaitHTTPReady(targetListen, 10*time.Second); err != nil {
		return rollback(fmt.Errorf("Gateway service 已启动，但 Gateway 未 ready：%w", err))
	}

	if createdService {
		fmt.Println("ADM Gateway 已安装并启动")
	} else {
		fmt.Println("ADM Gateway 已更新并启动")
	}
	fmt.Println()
	fmt.Println("服务用户：", target.Name)
	fmt.Println("监听地址：", targetListen)
	fmt.Println("状态：    ready")
	fmt.Println("状态文件：", target.StatePath)
	fmt.Println("Host Policy：", strings.Join(result.Status.AllowedHosts, ", "))
	fmt.Println("Service Enabled：", status.Enabled)
	fmt.Println("Unit Version：", status.UnitVersion)
	if len(plan.Changes) > 0 {
		fmt.Println("本次变更：", strings.Join(plan.Changes, ", "))
	}
	fmt.Println()
	fmt.Println("Desktop:")
	fmt.Println("  URL:       http://<server-ip>:" + gatewayPort(targetListen))
	if result.AdminKeyGenerated {
		fmt.Println("  Admin Key:", result.AdminAPIKey)
	} else {
		fmt.Printf("  Admin Key: 已存在并保留（如已遗失原文，请运行 sudo -u %s -H adm gateway access rotate-admin-key）\n", target.Name)
	}
	fmt.Println()
	fmt.Println("Agent MCP:")
	fmt.Println("  URL:       http://<server-ip>:" + gatewayPort(targetListen) + "/mcp")
	if result.AgentKeyGenerated {
		fmt.Println("  Agent Key:", result.AgentAPIKey)
	} else {
		fmt.Printf("  Agent Key: 已存在并保留（如已遗失原文，请运行 sudo -u %s -H adm gateway access rotate-agent-key）\n", target.Name)
	}
	if result.AdminKeyGenerated || result.AgentKeyGenerated {
		fmt.Println()
		fmt.Println("新生成的密钥仅此次显示，请立即保存。")
	}
	fmt.Println()
	fmt.Println("Service:", status.UnitPath)
	return nil
}

func buildGatewayInstallPlan(existing gatewayservice.Status, target gatewayservice.TargetUser, listen, executable string, previous model.GatewayAccessSettings, hosts []string, rotateKeys bool) gatewayInstallPlan {
	action := "install"
	if existing.Installed {
		action = "upgrade"
	}
	adminKeyAction := "preserve"
	agentKeyAction := "preserve"
	if rotateKeys {
		adminKeyAction = "rotate"
		agentKeyAction = "rotate"
	} else {
		if strings.TrimSpace(previous.AdminAPIKeyHash) == "" {
			adminKeyAction = "generate"
		}
		if strings.TrimSpace(previous.AgentAPIKeyHash) == "" {
			agentKeyAction = "generate"
		}
	}

	changes := make([]string, 0)
	if !existing.Installed {
		changes = append(changes, "create_managed_service")
	} else {
		if !strings.EqualFold(strings.TrimSpace(existing.User), strings.TrimSpace(target.Name)) {
			changes = append(changes, "service_user")
		}
		if strings.TrimSpace(existing.Listen) != strings.TrimSpace(listen) {
			changes = append(changes, "listen")
		}
		if strings.TrimSpace(existing.StatePath) != "" && filepath.Clean(existing.StatePath) != filepath.Clean(target.StatePath) {
			changes = append(changes, "state_path")
		}
		if strings.TrimSpace(existing.Executable) != "" && filepath.Clean(existing.Executable) != filepath.Clean(executable) {
			changes = append(changes, "executable")
		}
		if existing.UnitVersion != gatewayservice.UnitVersion {
			changes = append(changes, "unit_version")
		}
		changes = append(changes, "restart_service")
	}
	if !sameGatewayHostSet(previous.AllowedHosts, hosts) {
		changes = append(changes, "allowed_hosts")
	}
	if adminKeyAction != "preserve" {
		changes = append(changes, "admin_api_key")
	}
	if agentKeyAction != "preserve" {
		changes = append(changes, "agent_api_key")
	}

	return gatewayInstallPlan{
		Action:             action,
		User:               target.Name,
		Listen:             listen,
		StatePath:          target.StatePath,
		Executable:         executable,
		AllowedHosts:       append([]string(nil), hosts...),
		AdminKeyAction:     adminKeyAction,
		AgentKeyAction:     agentKeyAction,
		ServiceEnabled:     !existing.Installed || existing.Enabled,
		CurrentUnitVersion: existing.UnitVersion,
		TargetUnitVersion:  gatewayservice.UnitVersion,
		Changes:            changes,
	}
}

func sameGatewayHostSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int, len(a))
	for _, value := range a {
		counts[strings.ToLower(strings.TrimSpace(value))]++
	}
	for _, value := range b {
		key := strings.ToLower(strings.TrimSpace(value))
		if counts[key] == 0 {
			return false
		}
		counts[key]--
	}
	for _, count := range counts {
		if count != 0 {
			return false
		}
	}
	return true
}

func resolveGatewayInstallTarget(existing gatewayservice.Status, userArg, listenArg string, port int) (string, string, error) {
	userName := strings.TrimSpace(userArg)
	if userName == "" {
		if existing.Installed && strings.TrimSpace(existing.User) != "" {
			userName = existing.User
		} else {
			return "", "", fmt.Errorf("首次 gateway install 需要 --user USER；已安装的 ADM service 后续升级可省略 --user")
		}
	}
	if strings.TrimSpace(listenArg) != "" && port != 0 {
		return "", "", fmt.Errorf("--listen 与 --port 不能同时使用")
	}
	listen := strings.TrimSpace(listenArg)
	if listen == "" {
		switch {
		case port != 0:
			if port < 1 || port > 65535 {
				return "", "", fmt.Errorf("无效 --port %d", port)
			}
			listen = net.JoinHostPort("0.0.0.0", strconv.Itoa(port))
		case existing.Installed && strings.TrimSpace(existing.Listen) != "":
			listen = existing.Listen
		default:
			listen = net.JoinHostPort("0.0.0.0", "8001")
		}
	}
	normalized, err := normalizeRemoteListen(listen)
	if err != nil {
		return "", "", err
	}
	return userName, normalized, nil
}

func resolveGatewayInstallHosts(previous []string, raw string, explicit bool) []string {
	if !explicit && len(previous) > 0 {
		return append([]string(nil), previous...)
	}
	return splitGatewayHosts(raw)
}

func validateGatewayServiceStartReadiness(status gatewayservice.Status) error {
	if !status.Installed {
		return fmt.Errorf("ADM Gateway service 尚未安装")
	}
	if !status.Managed {
		return fmt.Errorf("%s 不是 ADM 管理的 Gateway service；拒绝启动或重启", status.UnitPath)
	}
	if strings.TrimSpace(status.Listen) == "" {
		return fmt.Errorf("ADM Gateway service 缺少 listen 元数据；请重新运行 gateway install")
	}
	if strings.TrimSpace(status.StatePath) == "" {
		return fmt.Errorf("ADM Gateway service 缺少 state path 元数据；请重新运行 gateway install")
	}
	if err := validateGatewayStartReadiness(app.New(status.StatePath), status.Listen); err != nil {
		return fmt.Errorf("Gateway service 启动前检查失败：%w", err)
	}
	return nil
}

func waitGatewayServiceReady(status gatewayservice.Status) error {
	if strings.TrimSpace(status.Listen) == "" {
		return fmt.Errorf("Gateway service 未返回监听地址")
	}
	_, err := gateway.WaitHTTPReady(status.Listen, 10*time.Second)
	return err
}

func applyGatewayService(existing gatewayservice.Status, options gatewayservice.InstallOptions) (gatewayservice.Status, bool, error) {
	if !existing.Installed {
		status, err := gatewayservice.Install(options)
		return status, true, err
	}
	if !existing.Managed {
		return gatewayservice.Status{}, false, fmt.Errorf("%s is not managed by ADM; refusing to overwrite it", existing.UnitPath)
	}
	status, err := gatewayservice.Reconfigure(options)
	return status, false, err
}

func restoreGatewayService(previous gatewayservice.Status) error {
	if !previous.Installed {
		return nil
	}
	if !previous.Managed {
		return fmt.Errorf("%s is not managed by ADM", previous.UnitPath)
	}
	target, err := gatewayservice.ResolveTargetUser(previous.User)
	if err != nil {
		return err
	}
	if strings.TrimSpace(previous.StatePath) != "" {
		target.StatePath = previous.StatePath
	}
	_, err = gatewayservice.Reconfigure(gatewayservice.InstallOptions{
		User:       target,
		Listen:     previous.Listen,
		Executable: previous.Executable,
		Start:      previous.Active,
	})
	if err != nil {
		return err
	}
	if !previous.Enabled {
		_, err = gatewayservice.SetEnabled(false)
	}
	return err
}

func normalizeRemoteListen(raw string) (string, error) {
	listen := strings.TrimSpace(raw)
	if strings.HasPrefix(listen, ":") {
		listen = "0.0.0.0" + listen
	}
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "", fmt.Errorf("无效监听地址 %q：%w", raw, err)
	}
	if strings.TrimSpace(port) == "" {
		return "", fmt.Errorf("监听地址必须包含端口")
	}
	if host == "localhost" {
		return listen, nil
	}
	if ip := net.ParseIP(host); ip == nil && strings.TrimSpace(host) == "" {
		return "", fmt.Errorf("监听地址必须包含 host")
	}
	return listen, nil
}

func currentExecutablePath() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("获取当前 ADM 可执行文件路径: %w", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return "", err
	}
	return executable, nil
}

func printGatewayServiceStatus(status gatewayservice.Status) {
	fmt.Println("ADM Gateway Service")
	fmt.Println("平台支持：", status.Supported)
	fmt.Println("已安装：  ", status.Installed)
	if !status.Installed {
		return
	}
	fmt.Println("ADM 管理：", status.Managed)
	if status.Managed {
		fmt.Println("Unit 版本：", status.UnitVersion)
	}
	if !status.Managed {
		if status.Detail != "" {
			fmt.Println("详情：     ", status.Detail)
		}
		return
	}
	fmt.Println("Active：   ", status.Active)
	fmt.Println("Enabled：  ", status.Enabled)
	if status.PID > 0 {
		fmt.Println("PID：      ", status.PID)
	}
	fmt.Println("Unit：     ", status.UnitPath)
	fmt.Println("运行用户：", status.User)
	fmt.Println("监听：     ", status.Listen)
	fmt.Println("Executable：", status.Executable)
	fmt.Println("状态文件： ", status.StatePath)
	if status.Detail != "" {
		fmt.Println("详情：     ", status.Detail)
	}
}
