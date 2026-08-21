package member

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/siliconwitchery/superstack-cli/internal/api"
	"github.com/siliconwitchery/superstack-cli/internal/api/apitest"
)

func TestMemberAdd(t *testing.T) {
	tests := []struct {
		name       string
		refusal    string
		wantOutput string
		wantError  string
	}{
		{name: "added", wantOutput: "Gave member@example.com access to fleet 3.\n"},
		{name: "server refusal", refusal: "no such account", wantError: "no such account"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
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

				if test.refusal != "" {
					http.Error(w, test.refusal, http.StatusNotFound)
					return
				}

				w.WriteHeader(http.StatusNoContent)
			})

			session, out := apitest.LoggedInSession(t, mux)

			err := Add(session, []string{"member@example.com", "3"})

			if test.wantError != "" {
				if err == nil || err.Error() != test.wantError {
					t.Fatalf("error = %v, want %q", err, test.wantError)
				}
			} else if err != nil {
				t.Fatal(err)
			}

			if addedPath != "/fleets/3/members" || addedEmail != "member@example.com" {
				t.Errorf("the server saw %q added at %q, want %q at %q",
					addedEmail, addedPath, "member@example.com", "/fleets/3/members")
			}

			if out.String() != test.wantOutput {
				t.Errorf("output = %q, want %q", out.String(), test.wantOutput)
			}
		})
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
		err := Add(api.Session{}, test.arguments)

		if err == nil || !strings.Contains(err.Error(), test.wantError) {
			t.Errorf("%s: error = %v, want it to mention %q", test.name, err, test.wantError)
		}
	}
}

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

			session, out := apitest.LoggedInSession(t, mux)

			err := List(session, test.arguments)

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
		{name: "the question names the fleet", email: "member@example.com", answer: "n\n", wantShown: `Take away member@example.com's access to fleet "pilot"?`},
		{name: "the success line names the fleet", email: "member@example.com", answer: "y\n", wantRemoved: true, wantShown: `Removed member@example.com's access to fleet "pilot".`},
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

			session, out := apitest.LoggedInSession(t, mux)

			session.In = strings.NewReader(test.answer)

			err := Remove(session, []string{test.email, "3"})

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
		err := Remove(api.Session{}, test.arguments)

		if err == nil || !strings.Contains(err.Error(), test.wantError) {
			t.Errorf("%s: error = %v, want it to mention %q", test.name, err, test.wantError)
		}
	}
}
