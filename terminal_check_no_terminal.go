// +build js nacl plan9 wasip1

package logrus

import (
	"io"
)

func checkIfTerminal(w io.Writer) bool {
	return false
}
