package api_test

import (
	"strings"
	"testing"
	"unicode"

	"github.com/siliconwitchery/superstack-cli/internal/api"
)

func TestPrintable(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "plain text is untouched", text: "weather-north", want: "weather-north"},
		{name: "an empty string stays empty", text: "", want: ""},
		{name: "spaces are kept", text: "north south", want: "north south"},
		{name: "accented letters are kept", text: "Ekbläd-Fjärrsond", want: "Ekbläd-Fjärrsond"},
		{name: "other scripts are kept", text: "気象観測機", want: "気象観測機"},
		{name: "emoji are kept", text: "roof \U0001F6F0", want: "roof \U0001F6F0"},
		{name: "an erase-line sequence is escaped", text: "\x1b[2K\rhidden", want: `\x1b[2K\rhidden`},
		{name: "a newline is escaped", text: "north\nsouth", want: `north\nsouth`},
		{name: "a tab is escaped", text: "north\tsouth", want: `north\tsouth`},
		{name: "a bell is escaped", text: "north\asouth", want: `north\asouth`},
		{name: "a right-to-left override is escaped", text: "a\u202eb", want: "a\\u202eb"},
		{name: "a zero-width joiner is escaped", text: "a\u200db", want: "a\\u200db"},
		{name: "a quote is escaped", text: `my"device`, want: `my\"device`},
		{name: "a backslash is escaped", text: `my\device`, want: `my\\device`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := api.Printable(test.text)

			if got != test.want {
				t.Errorf("Printable(%q) = %q, want %q", test.text, got, test.want)
			}

			for _, letter := range got {
				if unicode.IsControl(letter) {
					t.Errorf("Printable(%q) left the control character %U in %q", test.text, letter, got)
				}
			}

			if strings.ContainsAny(got, "\x1b\r\n") {
				t.Errorf("Printable(%q) = %q, which a terminal would still act on", test.text, got)
			}
		})
	}
}
