package commands

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogout(t *testing.T) {
	tests := []struct {
		name           string
		arguments      []string
		storedKey      string
		serverDown     bool
		revokeStatus   int
		wantError      string
		wantRevocation bool
		wantKeyKept    bool
		wantShown      string
	}{
		{
			name:      "arguments are refused",
			arguments: []string{"now"},
			wantError: "logout takes no arguments",
		},
		{
			name:           "revokes and forgets the stored key",
			storedKey:      "ssk_test",
			revokeStatus:   http.StatusNoContent,
			wantRevocation: true,
			wantShown:      "Logged out.\n",
		},
		{
			name:      "nothing stored",
			wantShown: "Not logged in.\n",
		},
		{
			name:           "server refuses the revocation",
			storedKey:      "ssk_test",
			revokeStatus:   http.StatusServiceUnavailable,
			wantError:      "still logged in",
			wantRevocation: true,
			wantKeyKept:    true,
		},
		{
			name:        "server unreachable",
			storedKey:   "ssk_test",
			serverDown:  true,
			wantError:   "still logged in",
			wantKeyKept: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isolateKeyStorage(t)

			revokedKey := ""

			mux := http.NewServeMux()

			mux.HandleFunc("POST /logout", func(w http.ResponseWriter, r *http.Request) {
				revokedKey = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")

				w.WriteHeader(test.revokeStatus)
			})

			server := httptest.NewServer(mux)

			defer server.Close()

			if test.serverDown {
				server.Close()
			}

			out := &bytes.Buffer{}
			session := NewSession(server.URL, "test", strings.NewReader(""), out)

			path, err := keyPath()

			if err != nil {
				t.Fatal(err)
			}

			if test.storedKey != "" {
				err = os.MkdirAll(filepath.Dir(path), 0o700)

				if err != nil {
					t.Fatal(err)
				}

				err = os.WriteFile(path, []byte(test.storedKey+"\n"), 0o600)

				if err != nil {
					t.Fatal(err)
				}
			}

			err = Logout(session, test.arguments)

			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error = %v, want it to mention %q", err, test.wantError)
				}
			} else if err != nil {
				t.Fatal(err)
			}

			if test.wantRevocation && revokedKey != test.storedKey {
				t.Errorf("the server saw %q revoked, want %q", revokedKey, test.storedKey)
			}

			if !test.wantRevocation && revokedKey != "" {
				t.Errorf("the server saw a revocation for %q, want none", revokedKey)
			}

			_, statError := os.Stat(path)

			if test.wantKeyKept && statError != nil {
				t.Error("the stored key is gone although the revocation failed")
			}

			if !test.wantKeyKept && !os.IsNotExist(statError) {
				t.Error("the stored key still exists after logout")
			}

			if out.String() != test.wantShown {
				t.Errorf("output = %q, want %q", out.String(), test.wantShown)
			}
		})
	}
}
