//go:build !windows && !linux && !darwin

package cli

import "fmt"

func findListeningProcess(listen string) (int, string, error) {
	return 0, "", fmt.Errorf("当前平台暂不支持自动定位监听 %s 的旧版 Gateway 进程", listen)
}
