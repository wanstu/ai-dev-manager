package cli

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"ai-dev-manager-v2/internal/configpath"
	"ai-dev-manager-v2/internal/dotenv"
	"ai-dev-manager-v2/internal/store"
)

type gatewayProcessIdentity struct {
	User            string
	StateDir        string
	ClientConfigDir string
}

func printGatewayIdentity(pid int) {
	current, _ := user.Current()
	statePath, _ := store.DefaultPath()
	clientConfigDir, _ := configpath.Dir()
	currentName := ""
	if current != nil {
		currentName = current.Username
	}
	fmt.Println("当前 CLI 用户：", displayUnknown(currentName))
	fmt.Println("当前状态目录：", displayUnknown(filepath.Dir(statePath)))
	fmt.Println("当前客户端配置目录：", displayUnknown(clientConfigDir))
	if pid <= 0 {
		return
	}
	identity := inspectGatewayProcessIdentity(pid)
	if identity.User != "" {
		fmt.Println("Gateway 运行用户：", identity.User)
	}
	if identity.StateDir != "" {
		fmt.Println("Gateway 状态目录：", identity.StateDir)
	}
	if identity.ClientConfigDir != "" {
		fmt.Println("Gateway 客户端配置目录：", identity.ClientConfigDir)
	}
	if currentName != "" && identity.User != "" && !sameUserName(currentName, identity.User) {
		fmt.Println("警告：当前 CLI 与正在运行的 Gateway 使用不同用户；本次本地配置修改不会作用于同一套默认用户状态。")
		fmt.Printf("建议：sudo -u %s -H adm gateway ...\n", identity.User)
	}
}

func inspectGatewayProcessIdentity(pid int) gatewayProcessIdentity {
	if runtime.GOOS != "linux" || pid <= 0 {
		return gatewayProcessIdentity{}
	}
	result := gatewayProcessIdentity{}
	if data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid)); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(line, "Uid:") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if account, err := user.LookupId(fields[1]); err == nil {
					result.User = account.Username
				} else {
					result.User = fields[1]
				}
			}
			break
		}
	}
	if data, err := os.ReadFile(fmt.Sprintf("/proc/%d/environ", pid)); err == nil {
		values := map[string]string{}
		for _, item := range strings.Split(string(data), "\x00") {
			key, value, ok := strings.Cut(item, "=")
			if ok {
				values[key] = value
			}
		}
		if root := strings.TrimSpace(values["ADM_V2_HOME"]); root != "" {
			result.StateDir = root
		}
		if home := strings.TrimSpace(values["HOME"]); home != "" {
			if result.StateDir == "" {
				result.StateDir = filepath.Join(home, ".config", "ai-dev-manager-v2")
			}
			result.ClientConfigDir = filepath.Join(home, ".config", "adm")
		}
	}
	return result
}

func sameUserName(a, b string) bool {
	a = strings.TrimSpace(strings.ToLower(strings.ReplaceAll(a, "\\", "/")))
	b = strings.TrimSpace(strings.ToLower(strings.ReplaceAll(b, "\\", "/")))
	if i := strings.LastIndex(a, "/"); i >= 0 {
		a = a[i+1:]
	}
	if i := strings.LastIndex(b, "/"); i >= 0 {
		b = b[i+1:]
	}
	return a == b
}

func displayUnknown(value string) string {
	if strings.TrimSpace(value) == "" {
		return "未知"
	}
	return value
}

func printDotenvDiagnostics() {
	userPath, _ := configpath.File(dotenv.FileName)
	executable, _ := os.Executable()
	paths := []string{userPath, filepath.Join(filepath.Dir(executable), dotenv.FileName)}
	seen := map[string]bool{}
	fmt.Println("客户端 .env")
	for _, path := range paths {
		if path == "" || seen[filepath.Clean(path)] {
			continue
		}
		seen[filepath.Clean(path)] = true
		values, err := dotenv.ParseFile(path)
		if os.IsNotExist(err) {
			fmt.Println("  -", path, "（不存在）")
			continue
		}
		if err != nil {
			fmt.Println("  -", path, "（解析失败：", err, "）")
			continue
		}
		fmt.Println("  -", path)
		if strings.TrimSpace(values["ADM_V2_URL"]) != "" {
			fmt.Println("    ✓ ADM_V2_URL")
		}
		if strings.TrimSpace(values["ADM_ADMIN_API_KEY"]) != "" {
			fmt.Println("    ✓ ADM_ADMIN_API_KEY")
		}
		if strings.TrimSpace(values["ADM_V2_ADMIN_API_KEY"]) != "" {
			fmt.Println("    ⚠ ADM_V2_ADMIN_API_KEY：旧客户端变量，当前仍兼容；建议迁移为 ADM_ADMIN_API_KEY")
		}
		if strings.TrimSpace(values["ADM_V2_AGENT_API_KEY"]) != "" {
			fmt.Println("    ⚠ ADM_V2_AGENT_API_KEY：服务端不再从 .env 读取 Agent Key；请使用 gateway setup/rotate")
		}
		if strings.TrimSpace(values["ADMIN_KEY"]) != "" {
			fmt.Println("    ⚠ 发现 ADMIN_KEY；ADM 不读取它，客户端 Admin Key 请使用 ADM_ADMIN_API_KEY")
		}
		if strings.TrimSpace(values["AGENT_KEY"]) != "" {
			fmt.Println("    ⚠ 发现 AGENT_KEY；ADM 不读取它，服务端 Agent Key 请使用 gateway setup/rotate 管理")
		}
	}
}
