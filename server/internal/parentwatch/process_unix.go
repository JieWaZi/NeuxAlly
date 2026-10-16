//go:build !windows

package parentwatch

import (
	"errors"

	"golang.org/x/sys/unix"
)

func processExists(processID int) bool {
	err := unix.Kill(processID, 0)
	return err == nil || errors.Is(err, unix.EPERM)
}
