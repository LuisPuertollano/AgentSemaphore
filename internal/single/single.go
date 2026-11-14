// Package single makes sure only one AgentSemaphore runs per user session.
package single

// Acquire takes the single-instance lock. ok is false when another instance
// already holds it. Call release when exiting.
func Acquire() (release func(), ok bool, err error) { return acquire() }
