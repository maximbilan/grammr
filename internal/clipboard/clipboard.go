package clipboard

import (
	"github.com/atotto/clipboard"
)

// Paste reads text from the system clipboard
func Paste() (string, error) {
	return clipboard.ReadAll()
}

// Copy writes text to the system clipboard
func Copy(text string) error {
	return clipboard.WriteAll(text)
}
