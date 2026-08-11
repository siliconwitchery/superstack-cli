package commands

import (
	"net/http"
	"strings"
	"testing"
)

func TestMemberRemove(t *testing.T) {
	tests := []struct {
		name  string
		email string
	}{
		{"a plain address", "member@example.com"},
		{"an address with a hash", "a#b@example.com"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			removedFleet := ""
			removedEmail := ""

			mux := http.NewServeMux()

			mux.HandleFunc("DELETE /fleets/{id}/members/{email}", func(w http.ResponseWriter, r *http.Request) {
				removedFleet = r.PathValue("id")
				removedEmail = r.PathValue("email")

				w.WriteHeader(http.StatusNoContent)
			})

			loggedInTestServer(t, mux)

			err := MemberRemove([]string{test.email, "3"})

			if err != nil {
				t.Fatal(err)
			}

			if removedFleet != "3" || removedEmail != test.email {
				t.Errorf("the server saw %q removed from fleet %q, want %q from fleet %q",
					removedEmail, removedFleet, test.email, "3")
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
		err := MemberRemove(test.arguments)

		if err == nil || !strings.Contains(err.Error(), test.wantError) {
			t.Errorf("%s: error = %v, want it to mention %q", test.name, err, test.wantError)
		}
	}
}
