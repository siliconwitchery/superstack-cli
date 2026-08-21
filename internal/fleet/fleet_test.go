package fleet

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/siliconwitchery/superstack-cli/internal/api"
	"github.com/siliconwitchery/superstack-cli/internal/api/apitest"
)

func TestFleetCreate(t *testing.T) {
	tests := []struct {
		name       string
		refusal    string
		wantOutput string
		wantError  string
	}{
		{name: "created", wantOutput: "Created fleet \"field trial\" with id 5.\n"},
		{name: "server refusal", refusal: "the body must carry a name", wantError: "the body must carry a name"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			created := ""
			mux := http.NewServeMux()

			mux.HandleFunc("POST /fleets", func(w http.ResponseWriter, r *http.Request) {
				body := struct {
					Name string `json:"name"`
				}{}

				json.NewDecoder(r.Body).Decode(&body)
				created = body.Name

				if test.refusal != "" {
					http.Error(w, test.refusal, http.StatusBadRequest)
					return
				}

				fmt.Fprintf(w, `{"id": 5, "name": %q}`, body.Name)
			})

			session, out := apitest.LoggedInSession(t, mux)

			err := Create(session, []string{"field trial"})

			if test.wantError != "" {
				if err == nil || err.Error() != test.wantError {
					t.Fatalf("error = %v, want %q", err, test.wantError)
				}
			} else if err != nil {
				t.Fatal(err)
			}

			if created != "field trial" {
				t.Errorf("the server saw %q created, want %q", created, "field trial")
			}

			if out.String() != test.wantOutput {
				t.Errorf("output = %q, want %q", out.String(), test.wantOutput)
			}
		})
	}
}

func TestFleetCreateTakesOneName(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
	}{
		{"no arguments", nil},
		{"two words", []string{"field", "trial"}},
		{"an empty name", []string{""}},
	}

	for _, test := range tests {
		err := Create(api.Session{}, test.arguments)

		if err == nil || !strings.Contains(err.Error(), "takes one name") {
			t.Errorf("%s: error = %v, want the one-name hint", test.name, err)
		}
	}
}

func TestFleetList(t *testing.T) {
	tests := []struct {
		name       string
		arguments  []string
		fleets     string
		refusal    string
		wantShown  []string
		wantAbsent []string
		wantExact  string
		wantError  string
	}{
		{
			name:      "server refusal",
			refusal:   "fleets unavailable",
			wantError: "fleets unavailable",
		},
		{
			name:      "some fleets",
			fleets:    `[{"id":1,"name":"field trial","owner":true},{"id":2,"name":"rooftop","owner":false}]`,
			wantShown: []string{"ID", "NAME", "ROLE", "field trial", "owner", "rooftop", "member"},
		},
		{
			name:       "a name with control characters is escaped",
			fleets:     `[{"id":1,"name":"\u001b[2K\rhidden","owner":true}]`,
			wantShown:  []string{`\x1b[2K\rhidden`},
			wantAbsent: []string{"\x1b"},
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
			wantError: "fleet list takes no arguments",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mux := http.NewServeMux()

			mux.HandleFunc("GET /fleets", func(w http.ResponseWriter, r *http.Request) {
				if test.refusal != "" {
					http.Error(w, test.refusal, http.StatusServiceUnavailable)
					return
				}

				fmt.Fprint(w, test.fleets)
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

func TestFleetRename(t *testing.T) {
	tests := []struct {
		name       string
		refusal    string
		wantOutput string
		wantError  string
	}{
		{name: "renamed", wantOutput: "Renamed fleet 9 to \"pilot\".\n"},
		{name: "server refusal", refusal: "no such fleet", wantError: "no such fleet"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			renamedPath := ""
			renamedTo := ""
			mux := http.NewServeMux()

			mux.HandleFunc("PATCH /fleets/{id}", func(w http.ResponseWriter, r *http.Request) {
				body := struct {
					Name string `json:"name"`
				}{}

				json.NewDecoder(r.Body).Decode(&body)
				renamedPath = r.URL.Path
				renamedTo = body.Name

				if test.refusal != "" {
					http.Error(w, test.refusal, http.StatusNotFound)
					return
				}

				w.WriteHeader(http.StatusNoContent)
			})

			session, out := apitest.LoggedInSession(t, mux)

			err := Rename(session, []string{"9", " pilot "})

			if test.wantError != "" {
				if err == nil || err.Error() != test.wantError {
					t.Fatalf("error = %v, want %q", err, test.wantError)
				}
			} else if err != nil {
				t.Fatal(err)
			}

			if renamedPath != "/fleets/9" || renamedTo != "pilot" {
				t.Errorf("the server saw %q renamed to %q, want %q renamed to %q",
					renamedPath, renamedTo, "/fleets/9", "pilot")
			}

			if out.String() != test.wantOutput {
				t.Errorf("output = %q, want %q", out.String(), test.wantOutput)
			}
		})
	}
}

func TestFleetRenameArguments(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
		wantError string
	}{
		{"no arguments", nil, "takes a fleet id and a new name"},
		{"only an id", []string{"3"}, "takes a fleet id and a new name"},
		{"three arguments", []string{"3", "field", "trial"}, "takes a fleet id and a new name"},
		{"a wordy id", []string{"pilot", "rooftop"}, "shown by fleet list"},
		{"an empty name", []string{"3", ""}, "takes a fleet id and a new name"},
		{"a whitespace name", []string{"3", "   "}, "takes a fleet id and a new name"},
	}

	for _, test := range tests {
		err := Rename(api.Session{}, test.arguments)

		if err == nil || !strings.Contains(err.Error(), test.wantError) {
			t.Errorf("%s: error = %v, want it to mention %q", test.name, err, test.wantError)
		}
	}
}

func TestFleetTransfer(t *testing.T) {
	tests := []struct {
		name            string
		answer          string
		fleets          string
		refusal         string
		wantTransferred bool
		wantOutput      string
		wantError       string
	}{
		{
			name:            "confirmed with y",
			answer:          "y\n",
			wantTransferred: true,
			wantOutput:      "Hand fleet \"pilot\" to successor@example.com? They become the owner, and you lose access to the fleet, its devices and its credit. [y/N] Transferred fleet \"pilot\" to successor@example.com.\n",
		},
		{name: "confirmed with yes", answer: "YES\n", wantTransferred: true},
		{
			name:       "declined with n",
			answer:     "n\n",
			wantOutput: "Hand fleet \"pilot\" to successor@example.com? They become the owner, and you lose access to the fleet, its devices and its credit. [y/N] Nothing transferred.\n",
		},
		{name: "declined by default", answer: "\n"},
		{name: "closed input", answer: ""},
		{
			name:            "the server refuses after the confirmation",
			answer:          "y\n",
			refusal:         "no one with that email address has logged in yet",
			wantTransferred: true,
			wantError:       "no one with that email address has logged in yet",
		},
		{
			name:      "a fleet that is not yours",
			answer:    "y\n",
			fleets:    `[]`,
			wantError: "no such fleet",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			transferredPath := ""
			transferredTo := ""
			fleets := test.fleets

			if fleets == "" {
				fleets = `[{"id":3,"name":"pilot","owner":true}]`
			}

			mux := http.NewServeMux()

			mux.HandleFunc("GET /fleets", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, fleets)
			})

			mux.HandleFunc("POST /fleets/{id}/owner", func(w http.ResponseWriter, r *http.Request) {
				body := struct {
					Email string `json:"email"`
				}{}

				json.NewDecoder(r.Body).Decode(&body)
				transferredPath = r.URL.Path
				transferredTo = body.Email

				if test.refusal != "" {
					http.Error(w, test.refusal, http.StatusNotFound)
					return
				}

				w.WriteHeader(http.StatusNoContent)
			})

			session, out := apitest.LoggedInSession(t, mux)

			session.In = strings.NewReader(test.answer)

			err := Transfer(session, []string{"3", "successor@example.com"})

			printed := out.String()

			if test.wantError != "" {
				if err == nil || err.Error() != test.wantError {
					t.Fatalf("error = %v, want %q", err, test.wantError)
				}

				if strings.Contains(printed, "Transferred") {
					t.Errorf("the output %q says the fleet was handed over although it was not", printed)
				}
			} else if err != nil {
				t.Fatal(err)
			}

			if test.wantOutput != "" && printed != test.wantOutput {
				t.Errorf("output = %q, want %q", printed, test.wantOutput)
			}

			if test.wantTransferred && (transferredPath != "/fleets/3/owner" || transferredTo != "successor@example.com") {
				t.Errorf("the server saw %q handed to %q, want %q handed to %q",
					transferredPath, transferredTo, "/fleets/3/owner", "successor@example.com")
			}

			if !test.wantTransferred && transferredPath != "" {
				t.Errorf("the server saw %q handed over although the confirmation was declined", transferredPath)
			}
		})
	}
}

func TestFleetTransferArguments(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
		wantError string
	}{
		{"no arguments", nil, "takes a fleet id and an email address"},
		{"only an id", []string{"3"}, "takes a fleet id and an email address"},
		{"an empty address", []string{"3", ""}, "takes a fleet id and an email address"},
		{"a wordy id", []string{"pilot", "successor@example.com"}, "shown by fleet list"},
	}

	for _, test := range tests {
		err := Transfer(api.Session{}, test.arguments)

		if err == nil || !strings.Contains(err.Error(), test.wantError) {
			t.Errorf("%s: error = %v, want it to mention %q", test.name, err, test.wantError)
		}
	}
}

func TestFleetDelete(t *testing.T) {
	tests := []struct {
		name        string
		answer      string
		refusal     string
		wantDeleted bool
		wantError   string
	}{
		{name: "confirmed with y", answer: "y\n", wantDeleted: true},
		{name: "confirmed with yes", answer: "YES\n", wantDeleted: true},
		{name: "declined with n", answer: "n\n"},
		{name: "declined by default", answer: "\n"},
		{name: "closed input", answer: ""},
		{
			name:        "the server refuses after the confirmation",
			answer:      "y\n",
			refusal:     "only the fleet's owner can delete it",
			wantDeleted: true,
			wantError:   "only the fleet's owner can delete it",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deletedPath := ""

			mux := http.NewServeMux()

			mux.HandleFunc("GET /fleets", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `[{"id":3,"name":"pilot","owner":true}]`)
			})

			mux.HandleFunc("GET /balance", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `[{"fleet":3,"balance":"0","currency":"eur"}]`)
			})

			mux.HandleFunc("DELETE /fleets/{id}", func(w http.ResponseWriter, r *http.Request) {
				deletedPath = r.URL.Path

				if test.refusal != "" {
					http.Error(w, test.refusal, http.StatusForbidden)
					return
				}

				w.WriteHeader(http.StatusNoContent)
			})

			session, out := apitest.LoggedInSession(t, mux)

			session.In = strings.NewReader(test.answer)

			err := Delete(session, []string{"3"})

			printed := out.String()

			switch {
			case test.wantError != "":
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error = %v, want it to mention %q", err, test.wantError)
				}

				if strings.Contains(printed, "Deleted") {
					t.Errorf("the output %q says the fleet was deleted although the server refused", printed)
				}

			case err != nil:
				t.Fatal(err)
			}

			if test.wantDeleted && deletedPath != "/fleets/3" {
				t.Errorf("the server saw %q deleted, want %q", deletedPath, "/fleets/3")
			}

			if test.wantDeleted && test.wantError == "" && !strings.Contains(printed, "Deleted fleet \"pilot\".") {
				t.Errorf("the output %q does not name the fleet it deleted", printed)
			}

			if !test.wantDeleted && deletedPath != "" {
				t.Errorf("the server saw %q deleted although the confirmation was declined", deletedPath)
			}
		})
	}
}

func TestFleetDeletePromptStatesForfeitedCredit(t *testing.T) {
	tests := []struct {
		name       string
		balance    string
		wantOutput string
		wantAbsent string
	}{
		{
			name:       "remaining credit is stated",
			balance:    `[{"fleet":3,"balance":"12.340000","currency":"eur"}]`,
			wantOutput: "Delete fleet \"pilot\", release its devices, and forfeit its remaining €12.34 of credit? It wipes their files and restarts their code, and claiming one again means pressing its pairing button in person. [y/N] Nothing deleted.\n",
		},
		{
			name:       "an empty balance stays quiet",
			balance:    `[{"fleet":3,"balance":"0","currency":"eur"}]`,
			wantOutput: "Delete fleet \"pilot\" and release its devices? It wipes their files and restarts their code, and claiming one again means pressing its pairing button in person. [y/N] Nothing deleted.\n",
			wantAbsent: "forfeit",
		},
		{
			name:       "an unparseable balance warns without an amount",
			balance:    `[{"fleet":3,"balance":"15,00","currency":"eur"}]`,
			wantOutput: "Delete fleet \"pilot\", release its devices, and forfeit its remaining credit? It wipes their files and restarts their code, and claiming one again means pressing its pairing button in person. [y/N] Nothing deleted.\n",
			wantAbsent: "€",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mux := http.NewServeMux()

			mux.HandleFunc("GET /fleets", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `[{"id":3,"name":"pilot","owner":true}]`)
			})

			mux.HandleFunc("GET /balance", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, test.balance)
			})

			session, out := apitest.LoggedInSession(t, mux)

			session.In = strings.NewReader("n\n")

			err := Delete(session, []string{"3"})

			printed := out.String()

			if err != nil {
				t.Fatal(err)
			}

			if printed != test.wantOutput {
				t.Errorf("output = %q, want %q", printed, test.wantOutput)
			}

			if !strings.Contains(printed, "It wipes their files and restarts their code, and claiming one again means pressing its pairing button in person.") {
				t.Errorf("the prompt %q does not say what releasing the devices does to them", printed)
			}

			if test.wantAbsent != "" && strings.Contains(printed, test.wantAbsent) {
				t.Errorf("the prompt %q mentions %q although nothing is forfeited", printed, test.wantAbsent)
			}
		})
	}
}

func TestFleetDeleteRefusesWhenTheBalanceIsUnknown(t *testing.T) {
	deletedPath := ""

	mux := http.NewServeMux()

	mux.HandleFunc("GET /fleets", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"id":3,"name":"pilot","owner":true}]`)
	})

	mux.HandleFunc("GET /balance", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "the server could not read the balances", http.StatusServiceUnavailable)
	})

	mux.HandleFunc("DELETE /fleets/{id}", func(w http.ResponseWriter, r *http.Request) {
		deletedPath = r.URL.Path

		w.WriteHeader(http.StatusNoContent)
	})

	session, _ := apitest.LoggedInSession(t, mux)

	session.In = strings.NewReader("y\n")

	err := Delete(session, []string{"3"})

	if err == nil || !strings.Contains(err.Error(), "could not read the balances") {
		t.Fatalf("error = %v, want the server's balance refusal", err)
	}

	if deletedPath != "" {
		t.Errorf("the server saw %q deleted although the credit could not be stated", deletedPath)
	}
}

func TestFleetDeleteUnknownFleet(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /fleets", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[]`)
	})

	session, _ := apitest.LoggedInSession(t, mux)

	err := Delete(session, []string{"9"})

	if err == nil || !strings.Contains(err.Error(), "no such fleet") {
		t.Fatalf("error = %v, want no such fleet", err)
	}
}

func TestFleetDeleteArguments(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
		wantError string
	}{
		{"no arguments", nil, "takes a fleet id"},
		{"two arguments", []string{"3", "4"}, "takes a fleet id"},
		{"a wordy id", []string{"pilot"}, "shown by fleet list"},
	}

	for _, test := range tests {
		err := Delete(api.Session{}, test.arguments)

		if err == nil || !strings.Contains(err.Error(), test.wantError) {
			t.Errorf("%s: error = %v, want it to mention %q", test.name, err, test.wantError)
		}
	}
}
