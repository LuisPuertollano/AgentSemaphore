package desktop

import (
	"net/url"
	"os/exec"
	"runtime"
)

// OpenURL opens an https link in the default browser.
func OpenURL(link string) error {
	u, err := url.Parse(link)
	if err != nil || u.Scheme != "https" {
		return ErrUnsupported
	}
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", u.String()).Start()
	case "darwin":
		return exec.Command("open", u.String()).Start()
	}
	return exec.Command("xdg-open", u.String()).Start()
}
