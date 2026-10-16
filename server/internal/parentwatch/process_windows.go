//go:build windows

package parentwatch

import (
	"errors"

	"golang.org/x/sys/windows"
)

func processExists(processID int) bool {
	handle, err := windows.OpenProcess(
		windows.SYNCHRONIZE,
		false,
		uint32(processID),
	)
	if err != nil {
		return errors.Is(err, windows.ERROR_ACCESS_DENIED)
	}
	defer windows.CloseHandle(handle)
	status, err := windows.WaitForSingleObject(handle, 0)
	if err != nil {
		return true
	}
	return status == uint32(windows.WAIT_TIMEOUT)
}
