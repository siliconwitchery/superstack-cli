package main

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

func TestResolve(t *testing.T) {
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
		{arguments: []string{"member", "add", "someone@example.com"}, name: "member add", rest: []string{"someone@example.com"}, found: true},
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
		entry, rest, found := resolve(test.arguments)

		if found != test.found {
			t.Errorf("resolve(%q) found = %v, want %v", test.arguments, found, test.found)
			continue
		}

		if !found {
			continue
		}

		if entry.name != test.name {
			t.Errorf("resolve(%q) name = %q, want %q", test.arguments, entry.name, test.name)
		}

		// Compared element by element, not joined: joining would accept a
		// resolve that collapsed every argument into one string
		if !slices.Equal(rest, test.rest) {
			t.Errorf("resolve(%q) rest = %q, want %q", test.arguments, rest, test.rest)
		}
	}
}

func TestCommandTable(t *testing.T) {
	seen := map[string]bool{}

	for _, section := range sections {
		if section.title == "" {
			t.Error("a section has no title")
		}

		for _, entry := range section.commands {
			switch {
			case entry.name == "":
				t.Errorf("section %q has a command with no name", section.title)

			case entry.summary == "":
				t.Errorf("command %q has no summary", entry.name)

			case seen[entry.name]:
				t.Errorf("command %q is listed twice", entry.name)
			}

			seen[entry.name] = true
		}
	}
}

func TestOnlyPlannedCommandsAreUnimplemented(t *testing.T) {
	// A command with no run reports that it is not implemented yet, so listing
	// them here is what stops a built one silently losing its wiring. version
	// and help are answered by main's own switch rather than by a run.
	planned := map[string]bool{
		"device claim":   true,
		"device list":    true,
		"device rename":  true,
		"device release": true,
		"device start":   true,
		"device stop":    true,
		"device restart": true,
		"upload":         true,
		"download":       true,
		"dev":            true,
		"tail":           true,
		"version":        true,
		"help":           true,
	}

	for _, section := range sections {
		for _, entry := range section.commands {
			switch {
			case entry.run == nil && !planned[entry.name]:
				t.Errorf("%q has no run, so it reports itself unimplemented", entry.name)

			case entry.run != nil && planned[entry.name]:
				t.Errorf("%q is implemented now, so take it off the planned list", entry.name)
			}

			delete(planned, entry.name)
		}
	}

	for name := range planned {
		t.Errorf("%q is on the planned list but not in the table", name)
	}
}

func TestHelpListsEveryCommand(t *testing.T) {
	help := bytes.Buffer{}

	printHelp(&help)

	for _, section := range sections {
		if !strings.Contains(help.String(), section.title) {
			t.Errorf("help is missing the section %q", section.title)
		}

		for _, entry := range section.commands {
			if !strings.Contains(help.String(), entry.name) {
				t.Errorf("help is missing the command %q", entry.name)
			}

			if !strings.Contains(help.String(), entry.summary) {
				t.Errorf("help is missing the summary for %q", entry.name)
			}
		}
	}
}
