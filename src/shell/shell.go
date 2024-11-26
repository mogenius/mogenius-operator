package shell

import (
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

var tty bool = isatty.IsTerminal(os.Stdout.Fd())

const (
	Normal       = ""
	Reset        = "\033[m"
	Bold         = "\033[1m"
	Black        = "\033[30m"
	Red          = "\033[31m"
	Green        = "\033[32m"
	Yellow       = "\033[33m"
	Blue         = "\033[34m"
	Magenta      = "\033[35m"
	Cyan         = "\033[36m"
	White        = "\033[37m"
	BoldBlack    = "\033[1;30m"
	BoldRed      = "\033[1;31m"
	BoldGreen    = "\033[1;32m"
	BoldYellow   = "\033[1;33m"
	BoldBlue     = "\033[1;34m"
	BoldMagenta  = "\033[1;35m"
	BoldCyan     = "\033[1;36m"
	FaintBlack   = "\033[2;30m"
	FaintRed     = "\033[2;31m"
	FaintGreen   = "\033[2;32m"
	FaintYellow  = "\033[2;33m"
	FaintBlue    = "\033[2;34m"
	FaintMagenta = "\033[2;35m"
	FaintCyan    = "\033[2;36m"
	BgBlack      = "\033[40m"
	BgRed        = "\033[41m"
	BgGreen      = "\033[42m"
	BgYellow     = "\033[43m"
	BgBlue       = "\033[44m"
	BgMagenta    = "\033[45m"
	BgCyan       = "\033[46m"
	Faint        = "\033[2m"
	FaintItalic  = "\033[2;3m"
	Reverse      = "\033[7m"
)

// Colorize shell output with provided colorcodes if os.Stdout is a shell
func Colorize(msg string, colorCodes ...string) string {
	if tty {
		colors := strings.Join(colorCodes, "")
		return colors + msg + Reset
	} else {
		return msg
	}
}
