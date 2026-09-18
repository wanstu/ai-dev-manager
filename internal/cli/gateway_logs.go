package cli

import (
	"fmt"
	"os"

	"ai-dev-manager-v2/internal/app"
)

func runGatewayLogs(service *app.Service, args []string) error {
	if wantsHelp(args) {
		fmt.Fprintln(os.Stdout, "用法：adm gateway logs status")
		fmt.Fprintln(os.Stdout, "显示 ADM 持久日志目录、单文件轮转阈值和每级保留数量；不输出日志正文。")
		return nil
	}
	if len(args) == 0 {
		return fmt.Errorf("缺少 gateway logs 子命令；运行 adm gateway logs -h 查看帮助")
	}
	switch args[0] {
	case "status":
		if len(args) != 1 {
			return fmt.Errorf("gateway logs status 不接受额外参数")
		}
		return writeJSON(service.LoggingStatus())
	default:
		return fmt.Errorf("未知 gateway logs 命令 %q；运行 adm gateway logs -h 查看帮助", args[0])
	}
}
