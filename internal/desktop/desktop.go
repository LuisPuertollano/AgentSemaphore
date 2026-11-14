// Package desktop talks to the Claude Desktop app: bring it to the front and,
// on Windows, flash its taskbar button. Nothing here reads or changes Claude's
// own settings.
package desktop

import "errors"

// ErrUnsupported: there is no Claude Desktop app on this system (e.g. Linux).
var ErrUnsupported = errors.New("Claude Desktop is not available on this system")

// CanOpenClaude reports whether OpenClaude can work here.
func CanOpenClaude() bool { return canOpen }

// OpenClaude starts Claude Desktop or brings it to the front.
func OpenClaude() error { return openClaude() }

// CanFlash reports whether FlashClaude can work here (Windows only).
func CanFlash() bool { return canFlash }

// FlashClaude makes Claude Desktop's taskbar button flash until it is brought
// to the front. It does nothing if Claude is already in front or not running.
func FlashClaude() { flashClaude() }
