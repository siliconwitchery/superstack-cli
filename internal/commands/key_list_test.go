package commands

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestKeyList(t *testing.T) {
	fleets := `[{"id":3,"name":"crew","owner":true},` +
		`{"id":4,"name":"skunkworks","owner":false},` +
		`{"id":5,"name":"spares","owner":true}]`

	keys := `[{"id":1,"fleet":3,"label":"deploy server","suffix":"ab2de"},` +
		`{"id":2,"fleet":4,"label":"lab sensor","suffix":"f9hjk"}]`

	tests := []struct {
		name       string
		arguments  []string
		wantShown  []string
		wantHidden []string
		wantError  string
	}{
		{
			name:      "every fleet's keys",
			arguments: []string{},
			wantShown: []string{"ID  FLEET  FLEET NAME", "crew", "skunkworks", "...ab2de", "...f9hjk", "deploy server", "lab sensor"},
		},
		{
			name:       "one fleet's keys",
			arguments:  []string{"3"},
			wantShown:  []string{"ID  FLEET  FLEET NAME", "crew", "...ab2de"},
			wantHidden: []string{"skunkworks", "f9hjk", "lab sensor"},
		},
		{
			name:       "a fleet without keys",
			arguments:  []string{"5"},
			wantShown:  []string{"No keys yet"},
			wantHidden: []string{"ID  FLEET"},
		},
		{
			name:       "machine-readable output",
			arguments:  []string{"--json"},
			wantShown:  []string{`"suffix":"ab2de"`, `"fleet":4`},
			wantHidden: []string{"ID  FLEET"},
		},
		{
			name:       "the flag before the id",
			arguments:  []string{"--json", "3"},
			wantShown:  []string{`"id":1`},
			wantHidden: []string{`"id":2`, "ID  FLEET"},
		},
		{
			name:      "a fleet out of reach",
			arguments: []string{"9"},
			wantError: "no such fleet",
		},
		{
			name:      "two fleet ids",
			arguments: []string{"3", "4"},
			wantError: "takes at most one fleet id",
		},
		{
			name:      "a wordy id",
			arguments: []string{"pilot"},
			wantError: "shown by fleet list",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mux := http.NewServeMux()

			mux.HandleFunc("GET /fleets", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, fleets)
			})

			mux.HandleFunc("GET /keys", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, keys)
			})

			session, out := loggedInSession(t, mux)

			err := KeyList(session, test.arguments)

			printed := out.String()

			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error = %v, want it to mention %q", err, test.wantError)
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			for _, want := range test.wantShown {
				if !strings.Contains(printed, want) {
					t.Errorf("the output %q leaves out %q", printed, want)
				}
			}

			for _, hidden := range test.wantHidden {
				if strings.Contains(printed, hidden) {
					t.Errorf("the output %q shows %q, want it filtered out", printed, hidden)
				}
			}
		})
	}
}
