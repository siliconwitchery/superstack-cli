package commands

import (
	"testing"
)

func captureBrowserOpens(t *testing.T) chan string {
	t.Helper()

	opens := make(chan string, 8)

	previousOpenBrowser := openBrowser

	openBrowser = func(url string) { opens <- url }

	t.Cleanup(func() { openBrowser = previousOpenBrowser })

	return opens
}
