package cli

import (
	"fmt"
	"os"
	"strings"
)

func runExecBlacklist(service cliManagementBackend, args []string) error {
	if wantsHelp(args) {
		fmt.Fprintln(os.Stdout, `用法：
  adm exec blacklist add --executable NAME_OR_PATH
  adm exec blacklist remove --executable NAME_OR_PATH
  adm exec blacklist list

黑名单优先于 allowlist 与 Full Authorization；加入黑名单会自动从 allowlist 移除。
移出黑名单不会自动加入 allowlist。`)
		return nil
	}
	switch args[0] {
	case "add":
		fs := newFlagSet("exec blacklist add", func() {
			fmt.Fprintln(os.Stdout, "用法：adm exec blacklist add --executable NAME_OR_PATH")
		})
		executable := fs.String("executable", "", "程序名或绝对路径")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*executable) == "" {
			return fmt.Errorf("缺少 --executable；运行 adm exec blacklist add -h 查看帮助")
		}
		items, err := service.ExecBlock(*executable)
		if err != nil {
			return err
		}
		return writeJSON(items)
	case "remove":
		fs := newFlagSet("exec blacklist remove", func() {
			fmt.Fprintln(os.Stdout, "用法：adm exec blacklist remove --executable NAME_OR_PATH")
		})
		executable := fs.String("executable", "", "程序名或绝对路径")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*executable) == "" {
			return fmt.Errorf("缺少 --executable；运行 adm exec blacklist remove -h 查看帮助")
		}
		items, err := service.ExecUnblock(*executable)
		if err != nil {
			return err
		}
		return writeJSON(items)
	case "list":
		if len(args) != 1 {
			return fmt.Errorf("exec blacklist list 不接受额外参数")
		}
		items, err := service.ExecBlockList()
		if err != nil {
			return err
		}
		return writeJSON(items)
	default:
		return fmt.Errorf("未知 exec blacklist 命令 %q", args[0])
	}
}
