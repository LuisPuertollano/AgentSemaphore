//go:build windows

package single

import (
	"errors"

	"golang.org/x/sys/windows"
)

// The prototype uses Local\SemaforoClaude, so both can run side by side while testing.
const mutexName = `Local\AgentSemaphore`

func acquire() (func(), bool, error) {
	name, err := windows.UTF16PtrFromString(mutexName)
	if err != nil {
		return nil, false, err
	}
	h, err := windows.CreateMutex(nil, false, name)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		if h != 0 {
			windows.CloseHandle(h)
		}
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return func() { windows.CloseHandle(h) }, true, nil
}
