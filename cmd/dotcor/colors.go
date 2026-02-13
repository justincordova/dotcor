package main

// ANSI color codes for terminal output
const (
	// Text colors
	ColorReset   = "\033[0m"
	ColorRed     = "\033[31m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorBlue    = "\033[34m"
	ColorMagenta = "\033[35m"
	ColorCyan    = "\033[36m"
	ColorWhite   = "\033[37m"

	// Bold text
	ColorBold = "\033[1m"

	// Background colors
	BgRed    = "\033[41m"
	BgGreen  = "\033[42m"
	BgYellow = "\033[43m"
)

// SuccessPrefix returns a green [OK] prefix
func SuccessPrefix() string {
	return ColorGreen + "[OK]" + ColorReset
}

// ErrorPrefix returns a red [X] prefix
func ErrorPrefix() string {
	return ColorRed + "[X]" + ColorReset
}

// WarningPrefix returns a yellow [!] prefix
func WarningPrefix() string {
	return ColorYellow + "[!]" + ColorReset
}

// InfoPrefix returns a blue [INFO] prefix
func InfoPrefix() string {
	return ColorBlue + "[INFO]" + ColorReset
}

// SuccessCheckmark returns a green checkmark
func SuccessCheckmark() string {
	return ColorGreen + "✓" + ColorReset
}

// ErrorX returns a red X mark
func ErrorX() string {
	return ColorRed + "✗" + ColorReset
}

// Bold returns bold text
func Bold(text string) string {
	return ColorBold + text + ColorReset
}

// Green returns green text
func Green(text string) string {
	return ColorGreen + text + ColorReset
}

// Red returns red text
func Red(text string) string {
	return ColorRed + text + ColorReset
}

// Yellow returns yellow text
func Yellow(text string) string {
	return ColorYellow + text + ColorReset
}

// Cyan returns cyan text
func Cyan(text string) string {
	return ColorCyan + text + ColorReset
}
