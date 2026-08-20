package commands

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestFleetRename(t *testing.T) {
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

		w.WriteHeader(http.StatusNoContent)
	})

	session, out := loggedInSession(t, mux)

	err := FleetRename(session, []string{"9", " pilot "})

	if err != nil {
		t.Fatal(err)
	}

	if renamedPath != "/fleets/9" || renamedTo != "pilot" {
		t.Errorf("the server saw %q renamed to %q, want %q renamed to %q",
			renamedPath, renamedTo, "/fleets/9", "pilot")
	}

	if out.String() != "Renamed the fleet to \"pilot\".\n" {
		t.Errorf("output = %q", out.String())
	}
}

func TestFleetRenameArguments(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
		wantError string
	}{
		{"no arguments", nil, "takes a fleet id and a name"},
		{"only an id", []string{"3"}, "takes a fleet id and a name"},
		{"three arguments", []string{"3", "field", "trial"}, "takes a fleet id and a name"},
		{"a wordy id", []string{"pilot", "rooftop"}, "shown by fleet list"},
		{"an empty name", []string{"3", ""}, "takes a fleet id and a name"},
		{"a whitespace name", []string{"3", "   "}, "takes a fleet id and a name"},
	}

	for _, test := range tests {
		err := FleetRename(Session{}, test.arguments)

		if err == nil || !strings.Contains(err.Error(), test.wantError) {
			t.Errorf("%s: error = %v, want it to mention %q", test.name, err, test.wantError)
		}
	}
}
