//go:build !windows

package main

import "fmt"

func runApplication(opts launchOptions, resources *appResources) error {
	if !opts.Web {
		fmt.Println(" [系统] 当前平台使用 Web 模式；Windows 版本默认使用桌面 GUI")
	}
	return runWebApplication(resources)
}

func reportStartupError(err error) {
	fmt.Printf(" [错误] %v\n", err)
}
