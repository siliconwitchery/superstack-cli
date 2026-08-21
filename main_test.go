package main

import (
	"bytes"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/siliconwitchery/superstack-cli/internal/dispatch"
)

func TestCommandTable(t *testing.T) {
	seen := map[string]bool{}

	for _, section := range sections {
		if section.Title == "" {
			t.Error("a section has no title")
		}

		for _, entry := range section.Commands {
			switch {
			case entry.Name == "":
				t.Errorf("section %q has a command with no name", section.Title)

			case entry.Summary == "":
				t.Errorf("command %q has no summary", entry.Name)

			case seen[entry.Name]:
				t.Errorf("command %q is listed twice", entry.Name)
			}

			seen[entry.Name] = true
		}
	}
}

func TestOnlyPlannedCommandsAreUnimplemented(t *testing.T) {
	plannedCommands := map[string]bool{
		"device start":   true,
		"device stop":    true,
		"device restart": true,
		"upload":         true,
		"download":       true,
		"dev":            true,
		"tail":           true,
	}
	answeredByDispatch := map[string]bool{
		"version": true,
		"help":    true,
	}

	for _, section := range sections {
		for _, entry := range section.Commands {
			switch {
			case answeredByDispatch[entry.Name]:
				if entry.Run != nil {
					t.Errorf("%q is answered by dispatch but also has a run", entry.Name)
				}

				delete(answeredByDispatch, entry.Name)

			case entry.Run == nil && !plannedCommands[entry.Name]:
				t.Errorf("%q has no run, so it reports itself unavailable", entry.Name)

			case entry.Run != nil && plannedCommands[entry.Name]:
				t.Errorf("%q is implemented now, so take it off the planned list", entry.Name)
			}

			delete(plannedCommands, entry.Name)
		}
	}

	for name := range plannedCommands {
		t.Errorf("%q is on the planned list but not in the table", name)
	}

	for name := range answeredByDispatch {
		t.Errorf("%q is answered by dispatch but not in the table", name)
	}
}

func TestNoPartImportsAnother(t *testing.T) {
	const module = "github.com/siliconwitchery/superstack-cli/internal/"
	const fixtures = "api/apitest"

	// The graph docs/cli.md publishes: a part reaches api and nothing else,
	// and only main reaches dispatch.
	allowed := map[string][]string{
		"api":         {},
		"api/apitest": {"api"},
		"dispatch":    {"api"},
		"account":     {"api"},
		"device":      {"api"},
		"fleet":       {"api"},
		"key":         {"api"},
		"login":       {"api"},
		"member":      {"api"},
	}

	walk := func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		owner := filepath.ToSlash(strings.TrimPrefix(filepath.Dir(path), "internal"+string(filepath.Separator)))

		permitted, known := allowed[owner]

		if !known {
			t.Errorf("%s is a package the graph does not mention, add it to docs/cli.md and to this test", owner)
			return nil
		}

		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)

		if err != nil {
			return err
		}

		for _, imported := range file.Imports {
			target := strings.Trim(imported.Path.Value, `"`)

			if !strings.HasPrefix(target, module) {
				continue
			}

			target = strings.TrimPrefix(target, module)

			if target == owner || slices.Contains(permitted, target) {
				continue
			}

			if target == fixtures && strings.HasSuffix(path, "_test.go") {
				continue
			}

			t.Errorf("%s imports %s, which the layout does not allow", path, target)
		}

		return nil
	}

	err := filepath.WalkDir("internal", walk)

	if err != nil {
		t.Fatal(err)
	}
}

func TestTheTableWiresEveryCommandOffered(t *testing.T) {
	wired := []string{
		"account balance", "account delete", "account topup",
		"device claim", "device list", "device release", "device rename",
		"fleet create", "fleet delete", "fleet list", "fleet rename", "fleet transfer",
		"key create", "key list", "key revoke",
		"login", "logout",
		"member add", "member list", "member remove",
	}

	offered := []string{}

	for _, section := range sections {
		for _, entry := range section.Commands {
			if entry.Run != nil {
				offered = append(offered, entry.Name)
			}
		}
	}

	slices.Sort(offered)

	if !slices.Equal(offered, wired) {
		t.Errorf("the table wires %v, want %v", offered, wired)
	}
}

func TestHelpRendersTheRealTable(t *testing.T) {
	out := &bytes.Buffer{}

	err := dispatch.Dispatch(sections, version, []string{"help"}, strings.NewReader(""), out)

	if err != nil {
		t.Fatal(err)
	}

	for _, section := range sections {
		if !strings.Contains(out.String(), section.Title) {
			t.Errorf("help leaves out the %q section", section.Title)
		}

		for _, entry := range section.Commands {
			if !strings.Contains(out.String(), entry.Name) {
				t.Errorf("help leaves out %q", entry.Name)
			}
		}
	}
}

// Re-runs this test binary as a child so main's own exit codes are observed
// rather than the error its command returned.
func TestMainReportsFailureWithANonZeroExit(t *testing.T) {
	arguments, isChild := os.LookupEnv("SUPERSTACK_MAIN_ARGUMENTS")

	if isChild {
		os.Args = append([]string{"superstack"}, strings.Fields(arguments)...)

		main()

		return
	}

	tests := []struct {
		name      string
		arguments string
		wantCode  int
		wantSays  string
	}{
		{name: "no arguments", arguments: " ", wantCode: 0, wantSays: "Usage: superstack"},
		{name: "the version", arguments: "version", wantCode: 0},
		{name: "an unknown command", arguments: "nonsense", wantCode: 1, wantSays: "unknown command"},
		{name: "a command nobody has built yet", arguments: "tail 111111111111111", wantCode: 1, wantSays: "not available yet"},
		{name: "a command that needs a login", arguments: "fleet list", wantCode: 1, wantSays: "not logged in"},
		{name: "a flag with no value", arguments: "--server", wantCode: 1, wantSays: "needs an address"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()

			command := exec.Command(os.Args[0], "-test.run=TestMainReportsFailureWithANonZeroExit")

			command.Env = append(os.Environ(),
				"SUPERSTACK_MAIN_ARGUMENTS="+test.arguments,
				"HOME="+home,
				"XDG_STATE_HOME="+home,
				"AppData="+home,
			)

			output, err := command.CombinedOutput()

			code := 0
			exitError, wasExit := err.(*exec.ExitError)

			switch {
			case wasExit:
				code = exitError.ExitCode()

			case err != nil:
				t.Fatal(err)
			}

			if code != test.wantCode {
				t.Errorf("superstack %s exited %d, want %d, having said %q", test.arguments, code, test.wantCode, output)
			}

			if test.wantSays != "" && !strings.Contains(string(output), test.wantSays) {
				t.Errorf("superstack %s said %q, want it to mention %q", test.arguments, output, test.wantSays)
			}
		})
	}
}
