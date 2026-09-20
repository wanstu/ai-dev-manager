package cli

import (
	"fmt"
	"net"
	"os"
	"strings"

	"ai-dev-manager-v2/internal/app"
)

const defaultRemoteGatewayListen = "0.0.0.0:8001"

func runGatewaySetup(service *app.Service, args []string) error {
	fs := newFlagSet("gateway setup", func() {
		fmt.Fprintln(os.Stdout, "用法：adm gateway setup --remote [--listen HOST:PORT] [--hosts HOST1,HOST2] [--rotate-keys]")
		fmt.Fprintln(os.Stdout, "\n一次完成远程 Gateway 的 Host policy 与双 Key 初始化；重复执行默认保留已有 Host policy 和 Key。")
	})
	remote := fs.Bool("remote", false, "初始化远程 Gateway")
	listen := fs.String("listen", "", "计划使用的监听地址；--remote 未指定时默认 0.0.0.0:8001")
	hostsRaw := fs.String("hosts", "*", "允许的 Host/IP，逗号分隔；默认 *")
	rotate := fs.Bool("rotate-keys", false, "即使已配置也重新生成 Admin/Agent Key")
	if err := fs.Parse(args); err != nil {
		return flagError(err)
	}
	if fs.NArg() != 0 || !*remote {
		return fmt.Errorf("gateway setup 当前要求 --remote；运行 adm gateway setup -h 查看帮助")
	}
	targetListen := strings.TrimSpace(*listen)
	if targetListen == "" {
		targetListen = defaultRemoteGatewayListen
	} else if strings.HasPrefix(targetListen, ":") {
		targetListen = "0.0.0.0" + targetListen
	}
	if _, _, err := net.SplitHostPort(targetListen); err != nil {
		return fmt.Errorf("无效 --listen %q：%w", targetListen, err)
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
	fmt.Println("Host Policy：", strings.Join(result.Status.AllowedHosts, ", "))
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
	fmt.Println("启动：        adm gateway start --listen " + targetListen)
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

func validateGatewayStartReadiness(service *app.Service, listen string) error {
	host, _, err := net.SplitHostPort(strings.TrimSpace(listen))
	if err != nil {
		return fmt.Errorf("invalid gateway listen address %q: %w", listen, err)
	}
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip != nil && ip.IsLoopback() {
		return nil
	}
	readiness, err := service.GatewayRemoteReadiness()
	if err != nil {
		return err
	}
	if readiness.Ready {
		return nil
	}
	var lines []string
	if readiness.HostPolicyConfigured {
		lines = append(lines, "✓ Host Policy")
	} else {
		lines = append(lines, "✗ Host Policy")
	}
	if readiness.AdminAPIKeyConfigured {
		lines = append(lines, "✓ Admin API Key")
	} else {
		lines = append(lines, "✗ Admin API Key")
	}
	if readiness.AgentAPIKeyConfigured {
		lines = append(lines, "✓ Agent API Key")
	} else {
		lines = append(lines, "✗ Agent API Key")
	}
	return fmt.Errorf("无法监听 %s\n\n远程 Gateway 尚未就绪：\n%s\n\n运行：\n  adm gateway setup --remote --listen %s",
		listen, strings.Join(lines, "\n"), listen)
}
