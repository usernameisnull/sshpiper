//go:build linux

package ioconn_test

import (
	"os/exec"
	"testing"

	"github.com/tg123/sshpiper/libplugin/ioconn"
)

func TestDialCmd(t *testing.T) {
	// me: 这里不能是ls这种很快完成的命令
	// cat 命令在没有参数时会从标准输入（stdin）读取内容，并原样输出到标准输出（stdout），并且不会立即退出——它会一直等待输入，
	// 直到 stdin 被关闭（比如收到 EOF）。这使得它非常适合用于这种“模拟一个长期运行的子进程进行双向通信”的测试场景。
	cmd := exec.Command("cat")

	conn, _, err := ioconn.DialCmd(cmd)
	if err != nil {
		t.Errorf("DialCmd returned an error: %v", err)
	}
	defer conn.Close()

	go func() {
		_, _ = conn.Write([]byte("world"))
	}()

	buf := make([]byte, 5)
	_, _ = conn.Read(buf)

	if string(buf) != "world" {
		t.Errorf("unexpected string read: %v", string(buf))
	}
}
