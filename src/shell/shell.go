package shell

import (
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

var tty bool = isatty.IsTerminal(os.Stdout.Fd())

const (
	Normal      = ""
	Reset       = "\033[m"
	Bold        = "\033[1m"
	Black       = "\033[30m"
	Red         = "\033[31m"
	Green       = "\033[32m"
	Yellow      = "\033[33m"
	Blue        = "\033[34m"
	Magenta     = "\033[35m"
	Cyan        = "\033[36m"
	White       = "\033[37m"
	BoldCyan    = "\033[1;36m"
	FaintYellow = "\033[2;33m"
	BgBlack     = "\033[40m"
	BgRed       = "\033[41m"
	BgGreen     = "\033[42m"
	BgYellow    = "\033[43m"
	BgBlue      = "\033[44m"
	BgCyan      = "\033[46m"
	Faint       = "\033[2m"
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
