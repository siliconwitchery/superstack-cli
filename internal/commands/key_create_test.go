package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
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
			wantError: "the server said: no such fleet",
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

			session, out := loggedInSession(t, mux)

			err := KeyCreate(session, test.arguments)

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
		})
	}
}
