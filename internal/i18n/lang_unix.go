//go:build !windows && !darwin

package i18n

// systemLanguage reads the POSIX locale variables in their precedence order
// for messages.
func systemLanguage() string { return fromEnv() }
