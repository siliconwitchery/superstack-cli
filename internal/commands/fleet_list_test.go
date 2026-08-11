package commands

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestFleetList(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
		fleets    string
		wantError string
	}{
		{
			name:   "some fleets",
			fleets: `[{"id":1,"name":"field trial","owner":true},{"id":2,"name":"rooftop","owner":false}]`,
		},
		{
			name:   "no fleets",
			fleets: `[]`,
		},
		{
			name:      "machine-readable output",
			arguments: []string{"--json"},
			fleets:    `[{"id":1,"name":"field trial","owner":true}]`,
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

			err := FleetList(test.arguments)

			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error = %v, want it to mention %q", err, test.wantError)
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
