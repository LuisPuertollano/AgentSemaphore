//go:build windows

package desktop

import "testing"

// On a machine without Claude Desktop (CI), finding and flashing must be
// harmless no-ops, never a crash.
func TestFlashWithoutClaudeIsANoOp(t *testing.T) {
	if !CanFlash() || !CanOpenClaude() {
		t.Fatal("Windows must support flash and open")
	}
	_ = findClaudeWindow()
	FlashClaude()
}
