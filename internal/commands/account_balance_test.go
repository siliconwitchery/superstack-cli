package commands

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestAccountBalance(t *testing.T) {
	tests := []struct {
		name       string
		arguments  []string
		fleets     string
		balances   string
		wantLines  []string
		wantAbsent []string
		wantExact  string
		wantError  string
	}{
		{
			name:      "every fleet",
			arguments: []string{},
			fleets:    `[{"id":1,"name":"crew","owner":true},{"id":2,"name":"pilot","owner":false}]`,
			balances:  `[{"fleet":1,"balance":"15.000000","currency":"eur"},{"fleet":2,"balance":"0","currency":"eur"}]`,
			wantLines: []string{"ID", "NAME", "BALANCE", "crew", "€15.00", "pilot", "€0.00"},
		},
		{
			name:       "one fleet",
			arguments:  []string{"2"},
			fleets:     `[{"id":1,"name":"crew","owner":true},{"id":2,"name":"pilot","owner":false}]`,
			balances:   `[{"fleet":1,"balance":"15.000000","currency":"eur"},{"fleet":2,"balance":"0","currency":"eur"}]`,
			wantLines:  []string{"pilot", "€0.00"},
			wantAbsent: []string{"crew"},
		},
		{
			name:      "machine readable",
			arguments: []string{"--json"},
			fleets:    `[{"id":1,"name":"crew","owner":true}]`,
			balances:  `[{"fleet":1,"balance":"15.000000","currency":"eur"}]`,
			wantExact: `[{"fleet":1,"balance":"15.000000","currency":"eur"}]` + "\n",
		},
		{
			name:      "machine readable for one fleet",
			arguments: []string{"2", "--json"},
			fleets:    `[{"id":1,"name":"crew","owner":true},{"id":2,"name":"pilot","owner":false}]`,
			balances:  `[{"fleet":1,"balance":"15.000000","currency":"eur"},{"fleet":2,"balance":"0","currency":"eur"}]`,
			wantExact: `[{"fleet":2,"balance":"0","currency":"eur"}]` + "\n",
		},
		{
			name:      "machine readable with no fleets",
			arguments: []string{"--json"},
			fleets:    `[]`,
			balances:  `[]`,
			wantExact: "[]\n",
		},
		{
			name:      "no fleets",
			arguments: []string{},
			fleets:    `[]`,
			balances:  `[]`,
			wantLines: []string{"No fleets yet"},
		},
		{
			name:      "an unknown fleet",
			arguments: []string{"9"},
			fleets:    `[{"id":1,"name":"crew","owner":true}]`,
			balances:  `[]`,
			wantError: "no such fleet",
		},
		{
			name:      "a wordy id",
			arguments: []string{"crew"},
			wantError: "shown by fleet list",
		},
		{
			name:      "too many arguments",
			arguments: []string{"1", "2"},
			wantError: "at most one fleet id",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mux := http.NewServeMux()

			mux.HandleFunc("GET /fleets", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, test.fleets)
			})

			mux.HandleFunc("GET /balance", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, test.balances)
			})

			loggedInTestServer(t, mux)

			printed, err := captureStdout(t, func() error {
				return AccountBalance(test.arguments)
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

			for _, want := range test.wantLines {
				if !strings.Contains(printed, want) {
					t.Errorf("the output %q does not show %q", printed, want)
				}
			}

			for _, absent := range test.wantAbsent {
				if strings.Contains(printed, absent) {
					t.Errorf("the output %q shows %q although it was filtered out", printed, absent)
				}
			}
		})
	}
}
