package main

import (
	"testing"
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
