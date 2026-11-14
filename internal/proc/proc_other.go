//go:build !linux && !darwin && !windows

package proc

func name(pid int) (string, bool) { return "", false }
