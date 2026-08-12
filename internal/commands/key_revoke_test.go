package commands

import (
	"net/http"
	"strings"
	"testing"
)

func TestKeyRevoke(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
		wantPath  string
		refusal   string
		wantError string
	}{
		{
			name:      "revoke a key",
			arguments: []string{"3"},
			wantPath:  "/keys/3",
		},
		{
			name:      "a key out of reach",
			arguments: []string{"9"},
			wantPath:  "/keys/9",
			refusal:   "no such key",
			wantError: "the server said: no such key",
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
			mux := http.NewServeMux()

			mux.HandleFunc("DELETE /keys/{id}", func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != test.wantPath {
					t.Errorf("the request went to %s, want %s", r.URL.Path, test.wantPath)
				}

				if test.refusal != "" {
					http.Error(w, test.refusal, http.StatusNotFound)
					return
				}

				w.WriteHeader(http.StatusNoContent)
			})

			loggedInTestServer(t, mux)

			err := KeyRevoke(test.arguments)

			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error = %v, want it to mention %q", err, test.wantError)
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
