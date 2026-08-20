package commands

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestMemberRemove(t *testing.T) {
	tests := []struct {
		name        string
		email       string
		answer      string
		refusal     string
		wantRemoved bool
		wantShown   string
		wantError   string
	}{
		{name: "a plain address", email: "member@example.com", answer: "y\n", wantRemoved: true},
		{name: "an address with a hash", email: "a#b@example.com", answer: "yes\n", wantRemoved: true},
		{name: "the prompt names the fleet", email: "member@example.com", answer: "y\n", wantRemoved: true, wantShown: `access to "pilot"`},
		{name: "declined by default", email: "member@example.com", answer: "\n", wantShown: "Nothing removed"},
		{name: "declined with n", email: "member@example.com", answer: "n\n", wantShown: "Nothing removed"},
		{name: "closed input", email: "member@example.com", wantShown: "Nothing removed"},
		{name: "server refusal", email: "member@example.com", answer: "y\n", refusal: "only an owner can remove members", wantRemoved: true, wantError: "only an owner can remove members"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			removedFleet := ""
			removedEmail := ""

			mux := http.NewServeMux()

			mux.HandleFunc("GET /fleets", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `[{"id":3,"name":"pilot","owner":true}]`)
			})

			mux.HandleFunc("DELETE /fleets/{id}/members/{email}", func(w http.ResponseWriter, r *http.Request) {
				removedFleet = r.PathValue("id")
				removedEmail = r.PathValue("email")

				if test.refusal != "" {
					http.Error(w, test.refusal, http.StatusForbidden)
					return
				}

				w.WriteHeader(http.StatusNoContent)
			})

			session, out := loggedInSession(t, mux)

			session.In = strings.NewReader(test.answer)

			err := MemberRemove(session, []string{test.email, "3"})

			printed := out.String()

			if test.wantError != "" {
				if err == nil || err.Error() != test.wantError {
					t.Fatalf("error = %v, want %q", err, test.wantError)
				}
			} else if err != nil {
				t.Fatal(err)
			}

			switch {
			case test.wantRemoved && (removedFleet != "3" || removedEmail != test.email):
				t.Errorf("the server saw %q removed from fleet %q, want %q from fleet %q",
					removedEmail, removedFleet, test.email, "3")

			case !test.wantRemoved && removedEmail != "":
				t.Errorf("the server saw %q removed although the confirmation was declined", removedEmail)
			}

			if test.wantShown != "" && !strings.Contains(printed, test.wantShown) {
				t.Errorf("the output %q does not show %q", printed, test.wantShown)
			}
		})
	}
}

func TestMemberRemoveArguments(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
		wantError string
	}{
		{"no arguments", nil, "takes an email address and a fleet id"},
		{"only an address", []string{"member@example.com"}, "takes an email address and a fleet id"},
		{"an empty address", []string{"", "3"}, "takes an email address and a fleet id"},
		{"a wordy id", []string{"member@example.com", "pilot"}, "shown by fleet list"},
	}

	for _, test := range tests {
		err := MemberRemove(Session{}, test.arguments)

		if err == nil || !strings.Contains(err.Error(), test.wantError) {
			t.Errorf("%s: error = %v, want it to mention %q", test.name, err, test.wantError)
		}
	}
}
