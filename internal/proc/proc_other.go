//go:build !linux && !darwin && !windows

package proc

func name(pid int) (string, bool) { return "", false }

func startTicks(int) (uint64, bool) { return 0, false }

func domain() string { return "" }

func parent(int) (int, bool) { return 0, false }
