package key

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/siliconwitchery/superstack-cli/internal/api/apitest"
)

func TestKeyCreate(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
		wantPath  string
		wantLabel string
		refusal   string
		wantError string
	}{
		{
			name:      "a labelled key",
			arguments: []string{"3", "deploy server"},
			wantPath:  "/fleets/3/keys",
			wantLabel: "deploy server",
		},
		{
			name:      "no label",
			arguments: []string{"3"},
			wantError: "takes a fleet id and a label",
		},
		{
			name:      "an empty label",
			arguments: []string{"3", ""},
			wantError: "takes a fleet id and a label",
		},
		{
			name:      "no fleet id",
			arguments: []string{},
			wantError: "takes a fleet id and a label",
		},
		{
			name:      "too many words",
			arguments: []string{"3", "deploy", "server"},
			wantError: "takes a fleet id and a label",
		},
		{
			name:      "a wordy id",
			arguments: []string{"pilot", "deploy server"},
			wantError: "shown by fleet list",
		},
		{
			name:      "the server refuses",
			arguments: []string{"9", "doomed"},
			wantPath:  "/fleets/9/keys",
			refusal:   "no such fleet",
			wantError: "no such fleet",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mux := http.NewServeMux()

			mux.HandleFunc("POST /fleets/{id}/keys", func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != test.wantPath {
					t.Errorf("the request went to %s, want %s", r.URL.Path, test.wantPath)
				}

				if test.refusal != "" {
					http.Error(w, test.refusal, http.StatusNotFound)
					return
				}

				sent := struct {
					Label string `json:"label"`
				}{}

				err := json.NewDecoder(r.Body).Decode(&sent)

				if err != nil {
					t.Errorf("the request body could not be decoded: %v", err)
				}

				if sent.Label != test.wantLabel {
					t.Errorf("the request carried label %q, want %q", sent.Label, test.wantLabel)
				}

				fmt.Fprint(w, `{"id":1,"key":"ssf_testtesttestab2de"}`)
			})

			session, out := apitest.LoggedInSession(t, mux)

			err := Create(session, test.arguments)

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

			if !strings.Contains(printed, "ssf_testtesttestab2de") {
				t.Errorf("the output %q does not show the key", printed)
			}

			if !strings.Contains(printed, "you will not see it again") {
				t.Errorf("the output %q does not warn that the key cannot be shown again", printed)
			}

			if !strings.Contains(printed, "Created fleet key 1.") {
				t.Errorf("the output %q does not name the fleet key it created", printed)
			}
		})
	}
}

func TestKeyList(t *testing.T) {
	fleets := `[{"id":3,"name":"crew","owner":true},` +
		`{"id":4,"name":"skunkworks","owner":false},` +
		`{"id":5,"name":"spares","owner":true}]`

	keys := `[{"id":1,"fleet":3,"label":"deploy server","suffix":"ab2de"},` +
		`{"id":2,"fleet":4,"label":"lab sensor","suffix":"f9hjk"}]`

	tests := []struct {
		name       string
		arguments  []string
		wantShown  []string
		wantHidden []string
		wantError  string
		keys       string
	}{
		{
			name:      "every fleet's keys",
			arguments: []string{},
			wantShown: []string{"ID  FLEET  FLEET NAME", "crew", "skunkworks", "...ab2de", "...f9hjk", "deploy server", "lab sensor"},
		},
		{
			name:       "one fleet's keys",
			arguments:  []string{"3"},
			wantShown:  []string{"ID  FLEET  FLEET NAME", "crew", "...ab2de"},
			wantHidden: []string{"skunkworks", "f9hjk", "lab sensor"},
		},
		{
			name:      "no keys",
			arguments: []string{},
			wantShown: []string{"No fleet keys yet. Create one with key create."},
			keys:      `[]`,
		},
		{
			name:       "a fleet without keys",
			arguments:  []string{"5"},
			wantShown:  []string{"No fleet keys on that fleet yet."},
			wantHidden: []string{"ID  FLEET"},
		},
		{
			name:       "machine-readable output",
			arguments:  []string{"--json"},
			wantShown:  []string{`"suffix":"ab2de"`, `"fleet":4`},
			wantHidden: []string{"ID  FLEET"},
		},
		{
			name:       "the flag before the id",
			arguments:  []string{"--json", "3"},
			wantShown:  []string{`"id":1`},
			wantHidden: []string{`"id":2`, "ID  FLEET"},
		},
		{
			name:      "a fleet out of reach",
			arguments: []string{"9"},
			wantError: "no such fleet",
		},
		{
			name:      "two fleet ids",
			arguments: []string{"3", "4"},
			wantError: "takes at most one fleet id",
		},
		{
			name:      "a wordy id",
			arguments: []string{"pilot"},
			wantError: "shown by fleet list",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			servedKeys := test.keys

			if servedKeys == "" {
				servedKeys = keys
			}

			mux := http.NewServeMux()

			mux.HandleFunc("GET /fleets", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, fleets)
			})

			mux.HandleFunc("GET /keys", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, servedKeys)
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

			for _, want := range test.wantShown {
				if !strings.Contains(printed, want) {
					t.Errorf("the output %q leaves out %q", printed, want)
				}
			}

			for _, hidden := range test.wantHidden {
				if strings.Contains(printed, hidden) {
					t.Errorf("the output %q shows %q, want it filtered out", printed, hidden)
				}
			}
		})
	}
}

func TestKeyRevoke(t *testing.T) {
	tests := []struct {
		name        string
		arguments   []string
		answer      string
		refusal     string
		wantRevoked string
		wantShown   string
		wantError   string
	}{
		{
			name:        "revoke a key",
			arguments:   []string{"3"},
			answer:      "y\n",
			wantRevoked: "/keys/3",
			wantShown:   "Revoked fleet key \"production\".",
		},
		{
			name:      "declined by default",
			arguments: []string{"3"},
			answer:    "\n",
			wantShown: "Nothing revoked",
		},
		{
			name:      "declined with n",
			arguments: []string{"3"},
			answer:    "n\n",
			wantShown: "Nothing revoked",
		},
		{
			name:      "closed input",
			arguments: []string{"3"},
			wantShown: "Nothing revoked",
		},
		{
			name:        "the server refuses after the confirmation",
			arguments:   []string{"3"},
			answer:      "y\n",
			refusal:     "no such key",
			wantRevoked: "/keys/3",
			wantError:   "no such key",
		},
		{
			name:      "a key that is not yours",
			arguments: []string{"9"},
			answer:    "y\n",
			wantError: "no such key",
		},
		{
			name:      "no key id",
			arguments: []string{},
			wantError: "takes a key id",
		},
		{
			name:      "two key ids",
			arguments: []string{"3", "4"},
			wantError: "takes a key id",
		},
		{
			name:      "a wordy id",
			arguments: []string{"pilot"},
			wantError: "shown by key list",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			revokedPath := ""

			mux := http.NewServeMux()

			mux.HandleFunc("GET /keys", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `[{"id":3,"fleet":1,"label":"production","suffix":"a1b2c"}]`)
			})

			mux.HandleFunc("DELETE /keys/{id}", func(w http.ResponseWriter, r *http.Request) {
				revokedPath = r.URL.Path

				if test.refusal != "" {
					http.Error(w, test.refusal, http.StatusNotFound)
					return
				}

				w.WriteHeader(http.StatusNoContent)
			})

			session, out := apitest.LoggedInSession(t, mux)
			session.In = strings.NewReader(test.answer)

			err := Revoke(session, test.arguments)

			printed := out.String()

			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error = %v, want it to mention %q", err, test.wantError)
				}
			} else if err != nil {
				t.Fatal(err)
			}

			if revokedPath != test.wantRevoked {
				t.Errorf("the server saw %q revoked, want %q", revokedPath, test.wantRevoked)
			}

			if test.wantShown != "" && !strings.Contains(printed, test.wantShown) {
				t.Errorf("the output %q does not show %q", printed, test.wantShown)
			}
		})
	}
}
