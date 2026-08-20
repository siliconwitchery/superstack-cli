package commands

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestFleetRename(t *testing.T) {
	tests := []struct {
		name       string
		refusal    string
		wantOutput string
		wantError  string
	}{
		{name: "renamed", wantOutput: "Renamed the fleet to \"pilot\".\n"},
		{name: "server refusal", refusal: "no such fleet", wantError: "no such fleet"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			renamedPath := ""
			renamedTo := ""
			mux := http.NewServeMux()

			mux.HandleFunc("PATCH /fleets/{id}", func(w http.ResponseWriter, r *http.Request) {
				body := struct {
					Name string `json:"name"`
				}{}

				json.NewDecoder(r.Body).Decode(&body)
				renamedPath = r.URL.Path
				renamedTo = body.Name

				if test.refusal != "" {
					http.Error(w, test.refusal, http.StatusNotFound)
					return
				}

				w.WriteHeader(http.StatusNoContent)
			})

			session, out := loggedInSession(t, mux)

			err := FleetRename(session, []string{"9", " pilot "})

			if test.wantError != "" {
				if err == nil || err.Error() != test.wantError {
					t.Fatalf("error = %v, want %q", err, test.wantError)
				}
			} else if err != nil {
				t.Fatal(err)
			}

			if renamedPath != "/fleets/9" || renamedTo != "pilot" {
				t.Errorf("the server saw %q renamed to %q, want %q renamed to %q",
					renamedPath, renamedTo, "/fleets/9", "pilot")
			}

			if out.String() != test.wantOutput {
				t.Errorf("output = %q, want %q", out.String(), test.wantOutput)
			}
		})
	}
}

func TestFleetRenameArguments(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
		wantError string
	}{
		{"no arguments", nil, "takes a fleet id and a new name"},
		{"only an id", []string{"3"}, "takes a fleet id and a new name"},
		{"three arguments", []string{"3", "field", "trial"}, "takes a fleet id and a new name"},
		{"a wordy id", []string{"pilot", "rooftop"}, "shown by fleet list"},
		{"an empty name", []string{"3", ""}, "takes a fleet id and a new name"},
		{"a whitespace name", []string{"3", "   "}, "takes a fleet id and a new name"},
	}

	for _, test := range tests {
		err := FleetRename(Session{}, test.arguments)

		if err == nil || !strings.Contains(err.Error(), test.wantError) {
			t.Errorf("%s: error = %v, want it to mention %q", test.name, err, test.wantError)
		}
	}
}
