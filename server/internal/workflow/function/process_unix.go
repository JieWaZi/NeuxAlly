//go:build !windows

package function

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func configureGroup(command *exec.Cmd) { command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }
func killGroup(command *exec.Cmd)      { _ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL) }
func processRSS(ctx, effectContext context.Context, pid string) (int64, error) {
	output, err := exec.CommandContext(ctx, "ps", "-axo", "pgid=,rss=").Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok && effectContext.Err() == nil {
			return 0, nil
		}
		return 0, memorySampleError(effectContext, err)
	}
	var total int64
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] != pid {
			continue
		}
		value, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0, hostError("memory_sample_parse", err)
		}
		total += value * 1024
	}
	return total, nil
}
