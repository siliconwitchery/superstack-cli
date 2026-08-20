package commands

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestMemberAdd(t *testing.T) {
	addedPath := ""
	addedEmail := ""

	mux := http.NewServeMux()

	mux.HandleFunc("POST /fleets/{id}/members", func(w http.ResponseWriter, r *http.Request) {
		body := struct {
			Email string `json:"email"`
		}{}

		json.NewDecoder(r.Body).Decode(&body)

		addedPath = r.URL.Path
		addedEmail = body.Email

		w.WriteHeader(http.StatusNoContent)
	})

	session, out := loggedInSession(t, mux)

	err := MemberAdd(session, []string{"member@example.com", "3"})

	if err != nil {
		t.Fatal(err)
	}

	if addedPath != "/fleets/3/members" || addedEmail != "member@example.com" {
		t.Errorf("the server saw %q added at %q, want %q at %q",
			addedEmail, addedPath, "member@example.com", "/fleets/3/members")
	}

	if out.String() != "Gave member@example.com access.\n" {
		t.Errorf("output = %q", out.String())
	}
}

func TestMemberAddArguments(t *testing.T) {
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
		err := MemberAdd(Session{}, test.arguments)

		if err == nil || !strings.Contains(err.Error(), test.wantError) {
			t.Errorf("%s: error = %v, want it to mention %q", test.name, err, test.wantError)
		}
	}
}
