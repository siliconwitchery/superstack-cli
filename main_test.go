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
	// A command with no run reports that it is not implemented yet, so listing
	// them here is what stops a built one silently losing its wiring. version
	// and help are answered by dispatch rather than by a run.
	planned := map[string]bool{
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
		for _, entry := range section.Commands {
			switch {
			case entry.Run == nil && !planned[entry.Name]:
				t.Errorf("%q has no run, so it reports itself unimplemented", entry.Name)

			case entry.Run != nil && planned[entry.Name]:
				t.Errorf("%q is implemented now, so take it off the planned list", entry.Name)
			}

			delete(planned, entry.Name)
		}
	}

	for name := range planned {
		t.Errorf("%q is on the planned list but not in the table", name)
	}
}
