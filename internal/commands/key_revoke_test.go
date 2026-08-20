package commands

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

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
			wantShown:   "production",
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

			session, out := loggedInSession(t, mux)
			session.In = strings.NewReader(test.answer)

			err := KeyRevoke(session, test.arguments)

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
