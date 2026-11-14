//go:build !windows && !darwin

package desktop

const (
	canOpen  = false
	canFlash = false
)

func openClaude() error { return ErrUnsupported }

func flashClaude() {}
