//go:build linux

package main

import (
	"os/exec"
	"syscall"

	reaper "github.com/ramr/go-reaper"
)

func init() {
	go reaper.Reap()
}

// me: 为一个即将执行的子进程（通过 *exec.Cmd 表示）设置“父进程死亡信号”（Parent Death Signal），即当父进程退出时，操作系统会自动向该子进程发送 SIGTERM 信号，使其有机会优雅退出或被终止
func setPdeathsig(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
}

func addProcessToJob(cmd *exec.Cmd) error {
	return nil
}
