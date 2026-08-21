package dispatch

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/siliconwitchery/superstack-cli/internal/api"
)

func TestTakeServerFlag(t *testing.T) {
	tests := []struct {
		name          string
		arguments     []string
		wantRemaining string
		wantBase      string
		wantError     string
	}{
		{
			name:          "no flag",
			arguments:     []string{"login"},
			wantRemaining: "login",
		},
		{
			name:          "a url",
			arguments:     []string{"--server", "http://localhost:8080", "login"},
			wantRemaining: "login",
			wantBase:      "http://localhost:8080",
		},
		{
			name:          "a url in equals form",
			arguments:     []string{"logout", "--server=https://staging.example.com"},
			wantRemaining: "logout",
			wantBase:      "https://staging.example.com",
		},
		{
			name:          "a trailing slash is trimmed",
			arguments:     []string{"--server", "http://localhost:8080/", "login"},
			wantRemaining: "login",
			wantBase:      "http://localhost:8080",
		},
		{
			name:          "the flag between command words",
			arguments:     []string{"fleet", "--server=http://localhost:9999", "list"},
			wantRemaining: "fleet list",
			wantBase:      "http://localhost:9999",
		},
		{
			name:      "a missing value",
			arguments: []string{"login", "--server"},
			wantError: "needs an address",
		},
		{
			name:      "an empty value",
			arguments: []string{"login", "--server="},
			wantError: "needs an address",
		},
		{
			name:      "a value that is only spaces",
			arguments: []string{"login", "--server", "   "},
			wantError: "needs an address",
		},
		{
			name:          "surrounding spaces are trimmed",
			arguments:     []string{"--server", "  http://localhost:8080/  ", "login"},
			wantRemaining: "login",
			wantBase:      "http://localhost:8080",
		},
		{
			name:      "an empty value from an unset shell variable",
			arguments: []string{"--server", "", "login"},
			wantError: "needs an address",
		},
		{
			name:      "an address of nothing but slashes",
			arguments: []string{"--server", "//", "login"},
			wantError: "needs an address",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			remaining, base, err := takeServerFlag(test.arguments)

			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error = %v, want it to mention %q", err, test.wantError)
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			if strings.Join(remaining, " ") != test.wantRemaining {
				t.Errorf("remaining = %q, want %q", strings.Join(remaining, " "), test.wantRemaining)
			}

			wantBase := test.wantBase

			if wantBase == "" {
				wantBase = api.DefaultBase
			}

			if base != wantBase {
				t.Errorf("base = %q, want %q", base, wantBase)
			}
		})
	}
}

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

func TestDispatch(t *testing.T) {
	sections := []Section{
		{Title: "Things", Commands: []Command{
			{Name: "thing list", Arguments: "<id>", Summary: "List a thing", Run: func(session api.Session, arguments []string) error {
				fmt.Fprintln(session.Out, "Listed.")
				return nil
			}},
			{Name: "thing pending", Summary: "Wait for a thing"},
		}},
		{Title: "Superstack", Commands: []Command{
			{Name: "version", Summary: "Show the version"},
			{Name: "help", Arguments: "[command]", Summary: "Show help"},
		}},
	}
	wantHelp := "superstack 1.2.3\n\n" +
		"Usage: superstack <command> [arguments]\n\n" +
		"Things\n" +
		"  thing list <id>  List a thing\n" +
		"  thing pending    Wait for a thing\n\n" +
		"Superstack\n" +
		"  version          Show the version\n" +
		"  help [command]   Show help\n"
	tests := []struct {
		name       string
		arguments  []string
		wantOutput string
		wantError  string
	}{
		{name: "no arguments", wantOutput: wantHelp},
		{name: "short help flag", arguments: []string{"-h"}, wantOutput: wantHelp},
		{name: "long help flag", arguments: []string{"--help"}, wantOutput: wantHelp},
		{name: "short version flag", arguments: []string{"-v"}, wantOutput: "1.2.3\n"},
		{name: "long version flag", arguments: []string{"--version"}, wantOutput: "1.2.3\n"},
		{name: "version command", arguments: []string{"version"}, wantOutput: "1.2.3\n"},
		{name: "help command", arguments: []string{"help"}, wantOutput: wantHelp},
		{name: "topic help", arguments: []string{"help", "thing", "list"}, wantOutput: "superstack thing list <id>\n\n  List a thing\n"},
		{name: "unknown help topic", arguments: []string{"help", "missing"}, wantError: "unknown command \"missing\"\nRun 'superstack help' for the list."},
		{name: "unknown command", arguments: []string{"missing"}, wantError: "unknown command \"missing\"\nRun 'superstack help' for the list."},
		{name: "unavailable command", arguments: []string{"thing", "pending"}, wantError: "thing pending is not available yet"},
		{name: "runnable command", arguments: []string{"thing", "list", "3"}, wantOutput: "Listed.\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out := &bytes.Buffer{}

			err := Dispatch(sections, "1.2.3", test.arguments, strings.NewReader(""), out)

			if test.wantError != "" {
				if err == nil || err.Error() != test.wantError {
					t.Fatalf("error = %v, want %q", err, test.wantError)
				}
			} else if err != nil {
				t.Fatal(err)
			}

			if out.String() != test.wantOutput {
				t.Errorf("output = %q, want %q", out.String(), test.wantOutput)
			}
		})
	}
}

func TestHelpListsEveryCommand(t *testing.T) {
	sections := []Section{
		{Title: "Things", Commands: []Command{{Name: "thing list", Arguments: "[--json]", Summary: "List things"}}},
		{Title: "Account", Commands: []Command{{Name: "account delete", Summary: "Delete the account"}}},
	}
	out := &bytes.Buffer{}
	session := api.NewSession(api.DefaultBase, "1.2.3", strings.NewReader(""), out)

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
