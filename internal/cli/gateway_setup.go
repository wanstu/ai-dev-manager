package cli

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"ai-dev-manager-v2/internal/app"
)

const defaultRemoteGatewayListen = "0.0.0.0:8001"

func runGatewaySetup(service *app.Service, args []string) error {
	fs := newFlagSet("gateway setup", func() {
		fmt.Fprintln(os.Stdout, "用法：adm gateway setup --remote [--port PORT] [--hosts HOST1,HOST2] [--rotate-keys]")
		fmt.Fprintln(os.Stdout, "\n一次完成远程 Gateway 的 Host policy 与双 Key 初始化；重复执行默认保留已有 Host policy 和 Key。")
	})
	remote := fs.Bool("remote", false, "初始化远程 Gateway")
	listen := fs.String("listen", "", "兼容旧脚本的监听地址；新配置仅需要 --port")
	port := fs.Int("port", 0, "服务端口；默认为 8001，监听 IP 固定 0.0.0.0")
	hostsRaw := fs.String("hosts", "*", "允许的 Host/IP，逗号分隔；默认 *")
	rotate := fs.Bool("rotate-keys", false, "即使已配置也重新生成 Admin/Agent Key")
	if err := fs.Parse(args); err != nil {
		return flagError(err)
	}
	if fs.NArg() != 0 || !*remote {
		return fmt.Errorf("gateway setup 当前要求 --remote；运行 adm gateway setup -h 查看帮助")
	}
	if flagWasSet(fs, "port") && flagWasSet(fs, "listen") {
		return fmt.Errorf("--port 与旧 --listen 不能同时指定")
	}
	portNumber := *port
	if portNumber != 0 && (portNumber < 1 || portNumber > 65535) {
		return fmt.Errorf("无效的服务端口：%d", portNumber)
	}
	targetListen := defaultRemoteGatewayListen
	if flagWasSet(fs, "port") {
		targetListen = fmt.Sprintf("0.0.0.0:%d", portNumber)
	}
	if flagWasSet(fs, "listen") {
		_, legacyPort, err := net.SplitHostPort(strings.TrimSpace(*listen))
		if err != nil {
			return fmt.Errorf("无效的旧监听地址：%w", err)
		}
		targetListen = net.JoinHostPort("0.0.0.0", legacyPort)
	}
	hosts := splitGatewayHosts(*hostsRaw)
	if !flagWasSet(fs, "hosts") {
		current, statusErr := service.GatewayAccessStatus()
		if statusErr != nil {
			return statusErr
		}
		if len(current.AllowedHosts) > 0 {
			hosts = append([]string(nil), current.AllowedHosts...)
		}
	}
	result, err := service.SetupGatewayRemote(hosts, *rotate)
	if err != nil {
		return err
	}

	fmt.Println("ADM Gateway 远程配置完成")
	fmt.Println()
	fmt.Println("监听地址：", targetListen)
	fmt.Println("白名单：   ", map[bool]string{true: "启用", false: "关闭"}[result.Status.WhitelistEnabled])
	fmt.Println("域名/目标：", strings.Join(result.Status.AllowedHosts, ", "))
	fmt.Println("来源 IP：  ", strings.Join(result.Status.AllowedClientIPs, ", "))
	fmt.Println("Admin API Key：已配置")
	fmt.Println("Agent API Key：已配置")
	if result.AdminKeyGenerated || result.AgentKeyGenerated {
		fmt.Println()
		fmt.Println("新生成的密钥仅此次显示，请保存：")
		if result.AdminKeyGenerated {
			fmt.Println("Admin Key：", result.AdminAPIKey)
		}
		if result.AgentKeyGenerated {
			fmt.Println("Agent Key：", result.AgentAPIKey)
		}
	} else {
		fmt.Println()
		fmt.Println("已有双 Key 已保留；如需轮换，请使用 adm gateway access rotate --all。")
	}
	fmt.Println()
	fmt.Println("Desktop URL： http://<server-ip>:" + gatewayPort(targetListen))
	fmt.Println("Agent MCP：   http://<server-ip>:" + gatewayPort(targetListen) + "/mcp")
	fmt.Println("启动：        adm gateway start --port " + gatewayPort(targetListen))
	return nil
}

func splitGatewayHosts(raw string) []string {
	var hosts []string
	for _, item := range strings.Split(raw, ",") {
		if value := strings.TrimSpace(item); value != "" {
			hosts = append(hosts, value)
		}
	}
	return hosts
}

func gatewayPort(listen string) string {
	_, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "8001"
	}
	return port
}

func validateGatewayStartReadiness(_ *app.Service, listen string) error {
	host, port, err := net.SplitHostPort(strings.TrimSpace(listen))
	if err != nil {
		return fmt.Errorf("无效的 Gateway 监听地址 %q: %w", listen, err)
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return fmt.Errorf("无效的 Gateway 端口 %q", port)
	}
	if host != "0.0.0.0" && host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return fmt.Errorf("Gateway 仅支持统一监听 0.0.0.0；旧脚本可暂用回环地址")
	}
	// Public binding does not bypass authentication: remote MCP requires both keys.
	return nil
}
