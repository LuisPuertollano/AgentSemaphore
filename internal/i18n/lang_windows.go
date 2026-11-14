//go:build windows

package i18n

import "syscall"

var procGetUserDefaultUILanguage = syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage")

// Primary language IDs (low 10 bits of a LANGID).
var primaryLang = map[uintptr]string{0x07: "de", 0x09: "en", 0x0a: "es"}

// systemLanguage returns the Windows display language of the current user.
func systemLanguage() string {
	if err := procGetUserDefaultUILanguage.Find(); err != nil {
		return ""
	}
	langID, _, _ := procGetUserDefaultUILanguage.Call()
	return primaryLang[langID&0x3ff]
}
