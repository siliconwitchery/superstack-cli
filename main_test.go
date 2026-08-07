package main

import (
	"bytes"
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
		{arguments: []string{"devices"}, name: "devices", rest: []string{}, found: true},
		{arguments: []string{"claim", "354820091234567", "sensor-01"}, name: "claim", rest: []string{"354820091234567", "sensor-01"}, found: true},
		{arguments: []string{"fleet", "create", "thermostats"}, name: "fleet create", rest: []string{"thermostats"}, found: true},
		{arguments: []string{"member", "add", "anna@example.com"}, name: "member add", rest: []string{"anna@example.com"}, found: true},
		{arguments: []string{"pipe"}, name: "pipe", rest: []string{}, found: true},
		{arguments: []string{"pipe", "set", "https://example.com/events"}, name: "pipe set", rest: []string{"https://example.com/events"}, found: true},
		{arguments: []string{"pipe", "rotate"}, name: "pipe rotate", rest: []string{}, found: true},
		{arguments: []string{"upload", "./main.lua", "--device", "sensor-01"}, name: "upload", rest: []string{"./main.lua", "--device", "sensor-01"}, found: true},
		{arguments: []string{"fleet"}, found: false},
		{arguments: []string{"member"}, found: false},
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

		if strings.Join(rest, " ") != strings.Join(test.rest, " ") {
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
