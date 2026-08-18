package commands

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
)

func answerOnStdin(t *testing.T, answer string) {
	t.Helper()

	readEnd, writeEnd, err := os.Pipe()

	if err != nil {
		t.Fatal(err)
	}

	originalStdin := os.Stdin

	os.Stdin = readEnd

	t.Cleanup(func() { os.Stdin = originalStdin })

	if answer != "" {
		_, err = writeEnd.WriteString(answer)

		if err != nil {
			t.Fatal(err)
		}
	}

	writeEnd.Close()
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

			loggedInTestServer(t, mux)

			answerOnStdin(t, test.answer)

			printed, err := captureStdout(t, func() error {
				return FleetDelete([]string{"3"})
			})

			switch {
			case test.wantError != "":
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error = %v, want it to mention %q", err, test.wantError)
				}

				// A refused delete must never claim the fleet is gone
				if strings.Contains(printed, "Deleted") {
					t.Errorf("the output %q says the fleet was deleted although the server refused", printed)
				}

			case err != nil:
				t.Fatal(err)
			}

			if test.wantDeleted && deletedPath != "/fleets/3" {
				t.Errorf("the server saw %q deleted, want %q", deletedPath, "/fleets/3")
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
		wantPrompt string
		wantAbsent string
	}{
		{
			name:       "remaining credit is stated",
			balance:    `[{"fleet":3,"balance":"12.340000","currency":"eur"}]`,
			wantPrompt: "forfeit its remaining €12.34 of credit",
		},
		{
			name:       "an empty balance stays quiet",
			balance:    `[{"fleet":3,"balance":"0","currency":"eur"}]`,
			wantAbsent: "forfeit",
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

			loggedInTestServer(t, mux)

			answerOnStdin(t, "n\n")

			printed, err := captureStdout(t, func() error {
				return FleetDelete([]string{"3"})
			})

			if err != nil {
				t.Fatal(err)
			}

			if test.wantPrompt != "" && !strings.Contains(printed, test.wantPrompt) {
				t.Errorf("the prompt %q does not state %q", printed, test.wantPrompt)
			}

			// Both prompts warn what the delete does to the devices
			if !strings.Contains(printed, "It erases them all, and claiming one again means pressing its button in person.") {
				t.Errorf("the prompt %q does not say the devices are erased", printed)
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

	loggedInTestServer(t, mux)

	answerOnStdin(t, "y\n")

	err := FleetDelete([]string{"3"})

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

	loggedInTestServer(t, mux)

	err := FleetDelete([]string{"9"})

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
		err := FleetDelete(test.arguments)

		if err == nil || !strings.Contains(err.Error(), test.wantError) {
			t.Errorf("%s: error = %v, want it to mention %q", test.name, err, test.wantError)
		}
	}
}
