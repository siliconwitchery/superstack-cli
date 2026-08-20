package commands

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestFetchKeysFailures(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		wantError string
	}{
		{name: "server refusal", status: http.StatusServiceUnavailable, body: "keys unavailable", wantError: "keys unavailable"},
		{name: "undecodable body", status: http.StatusOK, body: `{`, wantError: "unexpected EOF"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("GET /keys", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.status)
				fmt.Fprint(w, test.body)
			})

			session, _ := loggedInSession(t, mux)

			_, err := fetchKeys(session)

			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("error = %v, want it to mention %q", err, test.wantError)
			}
		})
	}
}
