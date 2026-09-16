package main

import (
	"fmt"
	"os"

	"ai-dev-manager-v2/internal/cli"
	"ai-dev-manager-v2/internal/dotenv"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "错误：", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if err := dotenv.LoadFromExecutableDir(); err != nil {
		return err
	}
	return cli.Run(args)
}
