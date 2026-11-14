//go:build !windows

package toasts

import "time"

// The notification database is Windows-only; elsewhere the source is absent.
type nonePlatform struct{}

// NewPlatform returns a platform that reports the source as absent.
func NewPlatform() Platform { return nonePlatform{} }

func (nonePlatform) Available() bool                  { return false }
func (nonePlatform) Changed() bool                    { return false }
func (nonePlatform) Read(time.Time) ([]Stored, error) { return nil, nil }
func (nonePlatform) ClaudeInForeground() bool         { return false }
