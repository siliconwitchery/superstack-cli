package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestFleetCreate(t *testing.T) {
	tests := []struct {
		name       string
		refusal    string
		wantOutput string
		wantError  string
	}{
		{name: "created", wantOutput: "Created fleet \"field trial\" with id 5.\n"},
		{name: "server refusal", refusal: "the body must carry a name", wantError: "the body must carry a name"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			created := ""
			mux := http.NewServeMux()

			mux.HandleFunc("POST /fleets", func(w http.ResponseWriter, r *http.Request) {
				body := struct {
					Name string `json:"name"`
				}{}

				json.NewDecoder(r.Body).Decode(&body)
				created = body.Name

				if test.refusal != "" {
					http.Error(w, test.refusal, http.StatusBadRequest)
					return
				}

				fmt.Fprintf(w, `{"id": 5, "name": %q}`, body.Name)
			})

			session, out := loggedInSession(t, mux)

			err := FleetCreate(session, []string{"field trial"})

			if test.wantError != "" {
				if err == nil || err.Error() != test.wantError {
					t.Fatalf("error = %v, want %q", err, test.wantError)
				}
			} else if err != nil {
				t.Fatal(err)
			}

			if created != "field trial" {
				t.Errorf("the server saw %q created, want %q", created, "field trial")
			}

			if out.String() != test.wantOutput {
				t.Errorf("output = %q, want %q", out.String(), test.wantOutput)
			}
		})
	}
}

func TestFleetCreateTakesOneName(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
	}{
		{"no arguments", nil},
		{"two words", []string{"field", "trial"}},
		{"an empty name", []string{""}},
	}

	for _, test := range tests {
		err := FleetCreate(Session{}, test.arguments)

		if err == nil || !strings.Contains(err.Error(), "takes one name") {
			t.Errorf("%s: error = %v, want the one-name hint", test.name, err)
		}
	}
}
