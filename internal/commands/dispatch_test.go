package commands

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

func TestResolve(t *testing.T) {
	sections := []Section{{Commands: []Command{
		{Name: "login"},
		{Name: "device list"},
		{Name: "device claim"},
		{Name: "fleet create"},
		{Name: "member add"},
		{Name: "key create"},
		{Name: "account balance"},
		{Name: "account topup"},
		{Name: "upload"},
	}}}

	tests := []struct {
		arguments []string
		name      string
		rest      []string
		found     bool
	}{
		{arguments: []string{"login"}, name: "login", rest: []string{}, found: true},
		{arguments: []string{"device", "list"}, name: "device list", rest: []string{}, found: true},
		{arguments: []string{"device", "claim", "354820091234567", "sensor-01"}, name: "device claim", rest: []string{"354820091234567", "sensor-01"}, found: true},
		{arguments: []string{"fleet", "create", "thermostats"}, name: "fleet create", rest: []string{"thermostats"}, found: true},
		{arguments: []string{"member", "add", "member@example.com"}, name: "member add", rest: []string{"member@example.com"}, found: true},
		{arguments: []string{"key", "create", "42", "production"}, name: "key create", rest: []string{"42", "production"}, found: true},
		{arguments: []string{"account", "balance"}, name: "account balance", rest: []string{}, found: true},
		{arguments: []string{"account", "topup", "42"}, name: "account topup", rest: []string{"42"}, found: true},
		{arguments: []string{"upload", "./main.lua", "--device", "sensor-01"}, name: "upload", rest: []string{"./main.lua", "--device", "sensor-01"}, found: true},
		{arguments: []string{"fleet"}, found: false},
		{arguments: []string{"member"}, found: false},
		{arguments: []string{"device"}, found: false},
		{arguments: []string{"key"}, found: false},
		{arguments: []string{"account"}, found: false},
		{arguments: []string{"deploy"}, found: false},
		{arguments: []string{}, found: false},
	}

	for _, test := range tests {
		entry, rest, found := resolve(sections, test.arguments)

		if found != test.found {
			t.Errorf("resolve(%q) found = %v, want %v", test.arguments, found, test.found)
			continue
		}

		if !found {
			continue
		}

		if entry.Name != test.name {
			t.Errorf("resolve(%q) name = %q, want %q", test.arguments, entry.Name, test.name)
		}

		if !slices.Equal(rest, test.rest) {
			t.Errorf("resolve(%q) rest = %q, want %q", test.arguments, rest, test.rest)
		}
	}
}

func TestHelpListsEveryCommand(t *testing.T) {
	sections := []Section{
		{Title: "Things", Commands: []Command{{Name: "thing list", Arguments: "[--json]", Summary: "List things"}}},
		{Title: "Account", Commands: []Command{{Name: "account delete", Summary: "Delete the account"}}},
	}
	out := &bytes.Buffer{}
	session := NewSession(defaultApiBase, "1.2.3", strings.NewReader(""), out)

	printHelp(session, sections)

	for _, section := range sections {
		if !strings.Contains(out.String(), section.Title) {
			t.Errorf("help is missing the section %q", section.Title)
		}

		for _, entry := range section.Commands {
			if !strings.Contains(out.String(), entry.Name) {
				t.Errorf("help is missing the command %q", entry.Name)
			}

			if !strings.Contains(out.String(), entry.Summary) {
				t.Errorf("help is missing the summary for %q", entry.Name)
			}
		}
	}
}
