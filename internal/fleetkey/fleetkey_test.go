package fleetkey

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/siliconwitchery/superstack-cli/internal/api/apitest"
)

func TestFleetKeyCreate(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
		wantPath  string
		wantLabel string
		answer    string
		refusal   string
		wantError string
	}{
		{
			name:      "the server answers without a fleet key",
			arguments: []string{"3", "deploy server"},
			wantPath:  "/fleets/3/fleet-keys",
			wantLabel: "deploy server",
			answer:    `{"id":1}`,
			wantError: "was not created",
		},
		{
			name:      "a labelled fleet key",
			arguments: []string{"3", "deploy server"},
			wantPath:  "/fleets/3/fleet-keys",
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
			wantPath:  "/fleets/9/fleet-keys",
			refusal:   "no such fleet",
			wantError: "no such fleet",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mux := http.NewServeMux()

			mux.HandleFunc("POST /fleets/{id}/fleet-keys", func(w http.ResponseWriter, r *http.Request) {
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

				answer := test.answer

				if answer == "" {
					answer = `{"id":1,"fleet_key":"ssf_testtesttestab2de"}`
				}

				fmt.Fprint(w, answer)
			})

			invocation, out := apitest.LoggedInInvocation(t, mux)

			err := Create(invocation, test.arguments)

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
				t.Errorf("the output %q does not show the fleet key", printed)
			}

			if !strings.Contains(printed, "you will not see it again") {
				t.Errorf("the output %q does not warn that the fleet key cannot be shown again", printed)
			}

			if !strings.Contains(printed, "Created key 1.") {
				t.Errorf("the output %q does not name the fleet key it created", printed)
			}
		})
	}
}

func TestFleetKeyList(t *testing.T) {
	fleets := `[{"id":3,"name":"crew","owner":true},` +
		`{"id":4,"name":"skunkworks","owner":false},` +
		`{"id":5,"name":"spares","owner":true}]`

	fleetKeys := `[{"id":1,"fleet_id":3,"label":"deploy server","fleet_key_suffix":"ab2de"},` +
		`{"id":2,"fleet_id":4,"label":"lab sensor","fleet_key_suffix":"f9hjk"}]`

	tests := []struct {
		name       string
		arguments  []string
		wantShown  []string
		wantHidden []string
		wantError  string
		fleetKeys  string
		refusal    string
	}{
		{
			name:      "server refusal",
			arguments: []string{},
			refusal:   "fleet keys unavailable",
			wantError: "fleet keys unavailable",
		},
		{
			name:      "every fleet's fleet keys",
			arguments: []string{},
			wantShown: []string{"ID  FLEET  FLEET NAME  KEY", "crew", "skunkworks", "...ab2de", "...f9hjk", "deploy server", "lab sensor"},
		},
		{
			name:       "one fleet's fleet keys",
			arguments:  []string{"3"},
			wantShown:  []string{"ID  FLEET  FLEET NAME", "crew", "...ab2de"},
			wantHidden: []string{"skunkworks", "f9hjk", "lab sensor"},
		},
		{
			name:       "a label with control characters is escaped",
			arguments:  []string{},
			fleetKeys:  `[{"id":1,"fleet_id":3,"label":"\u001b[2Kquiet","fleet_key_suffix":"ab2de"}]`,
			wantShown:  []string{`\x1b[2Kquiet`},
			wantHidden: []string{"\x1b"},
		},
		{
			name:      "no fleet keys",
			arguments: []string{},
			wantShown: []string{"No keys yet. Create one with key create."},
			fleetKeys: `[]`,
		},
		{
			name:       "a fleet without fleet keys",
			arguments:  []string{"5"},
			wantShown:  []string{"No keys on that fleet yet."},
			wantHidden: []string{"ID  FLEET"},
		},
		{
			name:       "machine-readable output",
			arguments:  []string{"--json"},
			wantShown:  []string{`"fleet_key_suffix":"ab2de"`, `"fleet_id":4`},
			wantHidden: []string{"ID  FLEET"},
		},
		{
			name:       "the flag before the id",
			arguments:  []string{"--json", "3"},
			wantShown:  []string{`"id":1`},
			wantHidden: []string{`"id":2`, "ID  FLEET"},
		},
		{
			name:      "a fleet the list does not name",
			arguments: []string{},
			fleetKeys: `[{"id":1,"fleet_id":99,"label":"orphan","fleet_key_suffix":"ab2de"}]`,
			wantShown: []string{"99     -"},
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
			servedFleetKeys := test.fleetKeys

			if servedFleetKeys == "" {
				servedFleetKeys = fleetKeys
			}

			mux := http.NewServeMux()

			mux.HandleFunc("GET /fleets", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, fleets)
			})

			mux.HandleFunc("GET /fleet-keys", func(w http.ResponseWriter, r *http.Request) {
				if test.refusal != "" {
					http.Error(w, test.refusal, http.StatusServiceUnavailable)
					return
				}

				fmt.Fprint(w, servedFleetKeys)
			})

			invocation, out := apitest.LoggedInInvocation(t, mux)

			err := List(invocation, test.arguments)

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

func TestFleetKeyRevoke(t *testing.T) {
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
			name:        "revoke a fleet key",
			arguments:   []string{"3"},
			answer:      "y\n",
			wantRevoked: "/fleet-keys/3",
			wantShown:   "Revoked key \"production\".",
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
			refusal:     "no such fleet key",
			wantRevoked: "/fleet-keys/3",
			wantError:   "no such fleet key",
		},
		{
			name:      "a fleet key that is not yours",
			arguments: []string{"9"},
			answer:    "y\n",
			wantError: "no such key",
		},
		{
			name:      "no fleet key id",
			arguments: []string{},
			wantError: "takes a key id",
		},
		{
			name:      "two fleet key ids",
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

			mux.HandleFunc("GET /fleet-keys", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `[{"id":3,"fleet_id":1,"label":"production","fleet_key_suffix":"a1b2c"}]`)
			})

			mux.HandleFunc("DELETE /fleet-keys/{id}", func(w http.ResponseWriter, r *http.Request) {
				revokedPath = r.URL.Path

				if test.refusal != "" {
					http.Error(w, test.refusal, http.StatusNotFound)
					return
				}

				w.WriteHeader(http.StatusNoContent)
			})

			invocation, out := apitest.LoggedInInvocation(t, mux)
			invocation.In = strings.NewReader(test.answer)

			err := Revoke(invocation, test.arguments)

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
