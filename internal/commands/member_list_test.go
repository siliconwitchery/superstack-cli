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
		wantFleet string
		wantShown []string
		wantExact string
		refusal   string
		wantError string
	}{
		{
			name:      "the people table",
			arguments: []string{"3"},
			people:    `{"owner":"owner@example.com","members":["member@example.com"]}`,
			wantFleet: "3",
			wantShown: []string{"EMAIL", "ROLE", "owner@example.com", "owner", "member@example.com", "member"},
		},
		{
			name:      "nobody but the owner",
			arguments: []string{"7"},
			people:    `{"owner":"owner@example.com","members":[]}`,
			wantFleet: "7",
			wantShown: []string{"owner@example.com", "owner"},
		},
		{
			name:      "machine-readable output",
			arguments: []string{"3", "--json"},
			people:    `{"owner":"owner@example.com","members":["member@example.com"]}`,
			wantFleet: "3",
			wantExact: `{"owner":"owner@example.com","members":["member@example.com"]}` + "\n",
		},
		{
			name:      "the flag before the id",
			arguments: []string{"--json", "5"},
			people:    `{"owner":"owner@example.com","members":[]}`,
			wantFleet: "5",
			wantExact: `{"owner":"owner@example.com","members":[]}` + "\n",
		},
		{
			name:      "server refusal",
			arguments: []string{"3"},
			wantFleet: "3",
			refusal:   "only members can see this fleet",
			wantError: "only members can see this fleet",
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

			askedFleet := ""

			mux.HandleFunc("GET /fleets/{id}/members", func(w http.ResponseWriter, r *http.Request) {
				askedFleet = r.PathValue("id")

				if test.refusal != "" {
					http.Error(w, test.refusal, http.StatusForbidden)
					return
				}

				fmt.Fprint(w, test.people)
			})

			session, out := loggedInSession(t, mux)

			err := MemberList(session, test.arguments)

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

			if askedFleet != test.wantFleet {
				t.Errorf("the people of fleet %q were listed, want fleet %q", askedFleet, test.wantFleet)
			}

			if test.wantExact != "" && printed != test.wantExact {
				t.Errorf("the output is %q, want exactly %q", printed, test.wantExact)
			}

			for _, want := range test.wantShown {
				if !strings.Contains(printed, want) {
					t.Errorf("the output %q does not show %q", printed, want)
				}
			}
		})
	}
}
