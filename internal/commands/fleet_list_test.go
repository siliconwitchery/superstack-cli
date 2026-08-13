package commands

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestFleetList(t *testing.T) {
	tests := []struct {
		name       string
		arguments  []string
		fleets     string
		wantShown  []string
		wantAbsent []string
		wantExact  string
		wantError  string
	}{
		{
			name:      "some fleets",
			fleets:    `[{"id":1,"name":"field trial","owner":true},{"id":2,"name":"rooftop","owner":false}]`,
			wantShown: []string{"ID", "NAME", "ROLE", "field trial", "owner", "rooftop", "member"},
		},
		{
			name:       "no fleets",
			fleets:     `[]`,
			wantShown:  []string{"No fleets yet"},
			wantAbsent: []string{"ID", "NAME"},
		},
		{
			name:      "machine-readable output",
			arguments: []string{"--json"},
			fleets:    `[{"id":1,"name":"field trial","owner":true}]`,
			wantExact: `[{"id":1,"name":"field trial","owner":true}]` + "\n",
		},
		{
			name:      "an unknown argument",
			arguments: []string{"--verbose"},
			wantError: "only --json",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mux := http.NewServeMux()

			mux.HandleFunc("GET /fleets", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, test.fleets)
			})

			loggedInTestServer(t, mux)

			printed, err := captureStdout(t, func() error {
				return FleetList(test.arguments)
			})

			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error = %v, want it to mention %q", err, test.wantError)
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			if test.wantExact != "" && printed != test.wantExact {
				t.Errorf("the output is %q, want exactly %q", printed, test.wantExact)
			}

			for _, want := range test.wantShown {
				if !strings.Contains(printed, want) {
					t.Errorf("the output %q does not show %q", printed, want)
				}
			}

			for _, absent := range test.wantAbsent {
				if strings.Contains(printed, absent) {
					t.Errorf("the output %q shows %q although there is nothing to list", printed, absent)
				}
			}
		})
	}
}
