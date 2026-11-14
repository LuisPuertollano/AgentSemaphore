//go:build linux

package proc

import (
	"os"
	"strconv"
	"strings"
)

func name(pid int) (string, bool) {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/comm")
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(b)), true
}
