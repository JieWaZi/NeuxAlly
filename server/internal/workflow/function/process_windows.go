//go:build windows

package function

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func configureGroup(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000200}
}
func killGroup(command *exec.Cmd) {
	_ = exec.Command("taskkill", "/PID", strconv.Itoa(command.Process.Pid), "/T", "/F").Run()
}
func processRSS(ctx, effectContext context.Context, pid string) (int64, error) {
	output, err := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "$p=Get-Process -Id "+pid+" -ErrorAction SilentlyContinue; if ($p) {$p.WorkingSet64} else {0}").Output()
	if err != nil {
		return 0, memorySampleError(effectContext, err)
	}
	value, err := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64)
	if err != nil {
		return 0, hostError("memory_sample_parse", err)
	}
	return value, nil
}
