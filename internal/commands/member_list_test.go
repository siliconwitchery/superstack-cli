package commands

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestMemberList(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
		people    string
		wantError string
	}{
		{
			name:      "the people table",
			arguments: []string{"3"},
			people:    `{"owner":"owner@example.com","members":["member@example.com"]}`,
		},
		{
			name:      "nobody but the owner",
			arguments: []string{"3"},
			people:    `{"owner":"owner@example.com","members":[]}`,
		},
		{
			name:      "machine-readable output",
			arguments: []string{"3", "--json"},
			people:    `{"owner":"owner@example.com","members":["member@example.com"]}`,
		},
		{
			name:      "the flag before the id",
			arguments: []string{"--json", "3"},
			people:    `{"owner":"owner@example.com","members":[]}`,
		},
		{
			name:      "no fleet id",
			arguments: []string{"--json"},
			wantError: "takes a fleet id",
		},
		{
			name:      "two fleet ids",
			arguments: []string{"3", "4"},
			wantError: "takes a fleet id",
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

			mux.HandleFunc("GET /fleets/{id}/members", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, test.people)
			})

			loggedInTestServer(t, mux)

			err := MemberList(test.arguments)

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
