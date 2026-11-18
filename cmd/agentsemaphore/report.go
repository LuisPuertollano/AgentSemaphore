package main

import (
	"net/url"
	"runtime"
	"strings"
)

// reportURL opens the "Test report" issue form with what the app knows filled
// in: the version, and the system in the title. Nothing is sent by itself; the
// user sees the form in the browser and decides.
func reportURL(version string) string {
	sys := map[string]string{"windows": "Windows", "darwin": "macOS", "linux": "Linux"}[runtime.GOOS]
	if sys == "" {
		sys = runtime.GOOS
	}
	arch := map[string]string{"amd64": "x64", "arm64": "ARM64"}[runtime.GOARCH]
	if arch == "" {
		arch = runtime.GOARCH
	}
	q := url.Values{}
	q.Set("template", "test-report.yml")
	q.Set("title", "Test report: "+sys+" "+arch+" / "+strings.TrimPrefix(version, "v"))
	q.Set("version", version)
	return "https://github.com/LuisPuertollano/AgentSemaphore/issues/new?" + q.Encode()
}
