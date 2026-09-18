package cli

import (
	"fmt"
	"os"
	"strings"

	"ai-dev-manager-v2/internal/app"
)

func runGatewayAccess(service *app.Service, args []string) error {
	if wantsHelp(args) {
		fmt.Fprintln(os.Stdout, `用法：
  adm gateway access status
  adm gateway access set-hosts --hosts HOST1,HOST2
  adm gateway access set-admin-key [--key KEY]
  adm gateway access clear-admin-key
  adm gateway access set-agent-key [--key KEY]
  adm gateway access clear-agent-key

远程 HTTP Gateway 必须同时配置 Host/IP 白名单、Admin API Key 与 Agent API Key。
Host 白名单支持 *，表示不限制 Host/IP、但两类 API Key 鉴权仍然强制；0.0.0.0 不是通配符。
set-admin-key 未提供 --key 时读取 ADM_V2_ADMIN_API_KEY。
set-agent-key 未提供 --key 时读取 ADM_V2_AGENT_API_KEY。
密钥不会写入命令输出。`)
		return nil
	}
	switch args[0] {
	case "status":
		if len(args) != 1 {
			return fmt.Errorf("gateway access status 不接受额外参数")
		}
		status, err := service.GatewayAccessStatus()
		if err != nil {
			return err
		}
		return writeJSON(status)
	case "set-hosts":
		fs := newFlagSet("gateway access set-hosts", func() {
			fmt.Fprintln(os.Stdout, "用法：adm gateway access set-hosts --hosts HOST1,HOST2")
		})
		raw := fs.String("hosts", "", "允许的远程 IP/域名，逗号分隔；* 表示任意 Host/IP；空值清空远程 Host 白名单")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("gateway access set-hosts 只接受 --hosts")
		}
		var hosts []string
		for _, item := range strings.Split(*raw, ",") {
			if value := strings.TrimSpace(item); value != "" {
				hosts = append(hosts, value)
			}
		}
		status, err := service.SetGatewayAllowedHosts(hosts)
		if err != nil {
			return err
		}
		return writeJSON(status)
	case "set-admin-key":
		value, err := gatewayAccessKeyArg("gateway access set-admin-key", "ADM_V2_ADMIN_API_KEY", args[1:])
		if err != nil {
			return err
		}
		status, err := service.SetGatewayAdminAPIKey(value)
		if err != nil {
			return err
		}
		return writeJSON(status)
	case "clear-admin-key":
		if len(args) != 1 {
			return fmt.Errorf("gateway access clear-admin-key 不接受额外参数")
		}
		status, err := service.ClearGatewayAdminAPIKey()
		if err != nil {
			return err
		}
		return writeJSON(status)
	case "set-agent-key":
		value, err := gatewayAccessKeyArg("gateway access set-agent-key", "ADM_V2_AGENT_API_KEY", args[1:])
		if err != nil {
			return err
		}
		status, err := service.SetGatewayAgentAPIKey(value)
		if err != nil {
			return err
		}
		return writeJSON(status)
	case "clear-agent-key":
		if len(args) != 1 {
			return fmt.Errorf("gateway access clear-agent-key 不接受额外参数")
		}
		status, err := service.ClearGatewayAgentAPIKey()
		if err != nil {
			return err
		}
		return writeJSON(status)
	default:
		return fmt.Errorf("未知 gateway access 命令 %q；运行 adm gateway access -h 查看帮助", args[0])
	}
}

func gatewayAccessKeyArg(command, envName string, args []string) (string, error) {
	fs := newFlagSet(command, func() {
		fmt.Fprintf(os.Stdout, "用法：adm %s [--key KEY]\n", command)
		fmt.Fprintf(os.Stdout, "未提供 --key 时读取 %s。\n", envName)
	})
	key := fs.String("key", "", "API Key（至少 16 字符；建议优先使用环境变量）")
	if err := fs.Parse(args); err != nil {
		return "", flagError(err)
	}
	if fs.NArg() != 0 {
		return "", fmt.Errorf("%s 只接受 --key", command)
	}
	value := strings.TrimSpace(*key)
	if value == "" {
		value = strings.TrimSpace(os.Getenv(envName))
	}
	if value == "" {
		return "", fmt.Errorf("API Key 不能为空；使用 --key 或 %s", envName)
	}
	return value, nil
}
