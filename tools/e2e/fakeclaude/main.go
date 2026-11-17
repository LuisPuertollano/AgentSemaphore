// Command fakeclaude stands in for a Claude Code process in end-to-end tests:
// built as "claude", it just waits, so the session source sees a live process
// with that name. (A renamed copy of `sleep` does not work everywhere: newer
// Linux images ship coreutils as one multi-call binary that dispatches on its
// own name.)
package main

import "time"

func main() { time.Sleep(10 * time.Minute) }
