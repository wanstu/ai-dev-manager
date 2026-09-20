package cli

import (
	"fmt"
	"os"
	"strings"

	"ai-dev-manager-v2/internal/app"
)

type gatewayAccessKeySetResult struct {
	app.GatewayAccessStatus
	GeneratedKey string `json:"generated_key,omitempty"`
}

func runGatewayAccess(service *app.Service, args []string) error {
	if wantsHelp(args) {
		fmt.Fprintln(os.Stdout, `用法：
  adm gateway access status
  adm gateway access set-hosts --hosts HOST1,HOST2
  adm gateway access rotate-admin-key
  adm gateway access rotate-agent-key
  adm gateway access rotate --all
  adm gateway access set-admin-key [--key KEY | --generate]
  adm gateway access clear-admin-key
  adm gateway access set-agent-key [--key KEY | --generate]
  adm gateway access clear-agent-key

远程 HTTP Gateway 必须同时配置 Host/IP 白名单、Admin API Key 与 Agent API Key。
Host 白名单支持 *，表示不限制 Host/IP、但两类 API Key 鉴权仍然强制；0.0.0.0 不是通配符。
set-admin-key / set-agent-key 必须显式使用 --key 或 --generate；服务端 Key 不会从 .env 隐式初始化。
--generate 使用加密安全随机源生成 256-bit Key，格式等价于 openssl rand -hex 32，并仅在本次命令结果中回显 generated_key。
手工提供的密钥不会写入命令输出。`)
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
	case "rotate-admin-key":
		if len(args) != 1 {
			return fmt.Errorf("gateway access rotate-admin-key 不接受额外参数")
		}
		result, err := service.RotateGatewayAdminAPIKey()
		if err != nil {
			return err
		}
		fmt.Println("Admin API Key 已轮换")
		fmt.Println("新 Key：", result.AdminAPIKey)
		fmt.Println("该 Key 仅显示此次；使用旧 Key 的 Desktop/CLI 将无法连接。")
		return nil
	case "rotate-agent-key":
		if len(args) != 1 {
			return fmt.Errorf("gateway access rotate-agent-key 不接受额外参数")
		}
		result, err := service.RotateGatewayAgentAPIKey()
		if err != nil {
			return err
		}
		fmt.Println("Agent API Key 已轮换")
		fmt.Println("新 Key：", result.AgentAPIKey)
		fmt.Println("该 Key 仅显示此次；使用旧 Key 的 Agent/MCP 客户端将无法连接。")
		return nil
	case "rotate":
		if len(args) != 2 || args[1] != "--all" {
			return fmt.Errorf("用法：adm gateway access rotate --all")
		}
		result, err := service.RotateGatewayAPIKeys()
		if err != nil {
			return err
		}
		fmt.Println("Admin / Agent API Key 已同时轮换")
		fmt.Println("Admin Key：", result.AdminAPIKey)
		fmt.Println("Agent Key：", result.AgentAPIKey)
		fmt.Println("两把 Key 均仅显示此次；旧凭据已失效。")
		return nil
	case "set-admin-key":
		value, generated, err := gatewayAccessKeyArg("gateway access set-admin-key", args[1:])
		if err != nil {
			return err
		}
		status, err := service.SetGatewayAdminAPIKey(value)
		if err != nil {
			return err
		}
		return writeGatewayAccessKeyResult(status, value, generated)
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
		value, generated, err := gatewayAccessKeyArg("gateway access set-agent-key", args[1:])
		if err != nil {
			return err
		}
		status, err := service.SetGatewayAgentAPIKey(value)
		if err != nil {
			return err
		}
		return writeGatewayAccessKeyResult(status, value, generated)
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

func writeGatewayAccessKeyResult(status app.GatewayAccessStatus, value string, generated bool) error {
	if !generated {
		return writeJSON(status)
	}
	return writeJSON(gatewayAccessKeySetResult{
		GatewayAccessStatus: status,
		GeneratedKey:        value,
	})
}

func gatewayAccessKeyArg(command string, args []string) (string, bool, error) {
	fs := newFlagSet(command, func() {
		fmt.Fprintf(os.Stdout, "用法：adm %s [--key KEY | --generate]\n", command)
		fmt.Fprintln(os.Stdout, "必须显式提供 --key 或 --generate；服务端不会从 .env 隐式读取 Key。")
		fmt.Fprintln(os.Stdout, "--generate 生成 256-bit 随机 Key，格式等价于 openssl rand -hex 32。")
	})
	key := fs.String("key", "", "API Key（至少 16 字符；服务端设置不会隐式读取 .env）")
	generate := fs.Bool("generate", false, "生成并设置 256-bit 安全随机 API Key；generated_key 仅在本次输出中回显")
	if err := fs.Parse(args); err != nil {
		return "", false, flagError(err)
	}
	if fs.NArg() != 0 {
		return "", false, fmt.Errorf("%s 只接受 --key 或 --generate", command)
	}
	value := strings.TrimSpace(*key)
	if *generate {
		if value != "" {
			return "", false, fmt.Errorf("%s 的 --key 与 --generate 不能同时使用", command)
		}
		generated, err := app.GenerateGatewayAPIKey()
		if err != nil {
			return "", false, err
		}
		return generated, true, nil
	}
	if value == "" {
		return "", false, fmt.Errorf("API Key 不能为空；使用 --key 或 --generate")
	}
	return value, false, nil
}
