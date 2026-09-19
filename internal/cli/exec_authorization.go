package cli

import (
	"fmt"
	"os"
	"strings"
)

func runExecAuthorization(service cliManagementBackend, args []string) error {
	if wantsHelp(args) {
		fmt.Fprintln(os.Stdout, `用法：
  adm exec authorization status
  adm exec authorization set --mode strict|full

strict：只允许显式 allowlist，并阻止 PowerShell/cmd/bash/sh 等通用命令解释器作为绕过入口。
full：允许未列入 allowlist 的 executable，但仍记录 full_authorization_bypass 审计。
两种模式都不能执行命令黑名单中的 executable。`)
		return nil
	}
	switch args[0] {
	case "status":
		if len(args) != 1 {
			return fmt.Errorf("exec authorization status 不接受额外参数")
		}
		status, err := service.ExecAuthorizationStatus()
		if err != nil {
			return err
		}
		return writeJSON(status)
	case "set":
		fs := newFlagSet("exec authorization set", func() {
			fmt.Fprintln(os.Stdout, "用法：adm exec authorization set --mode strict|full")
		})
		mode := fs.String("mode", "", "strict 或 full")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("exec authorization set 只接受 --mode")
		}
		switch strings.ToLower(strings.TrimSpace(*mode)) {
		case "strict":
			status, err := service.ExecFullAuthorizationSet(false)
			if err != nil {
				return err
			}
			return writeJSON(status)
		case "full":
			status, err := service.ExecFullAuthorizationSet(true)
			if err != nil {
				return err
			}
			return writeJSON(status)
		default:
			return fmt.Errorf("--mode 必须是 strict 或 full")
		}
	default:
		return fmt.Errorf("未知 exec authorization 命令 %q", args[0])
	}
}
