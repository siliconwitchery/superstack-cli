package commands

import (
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestAccountDelete(t *testing.T) {
	tests := []struct {
		name        string
		arguments   []string
		answer      string
		refusal     string
		refusalCode int
		wantDeleted bool
		wantShown   string
		wantError   string
	}{
		{
			name:        "confirmed with y",
			answer:      "y\n",
			wantDeleted: true,
			wantShown:   "Account deleted",
		},
		{
			name:        "confirmed with yes",
			answer:      "YES\n",
			wantDeleted: true,
			wantShown:   "Account deleted",
		},
		{
			name:      "declined by default",
			answer:    "\n",
			wantShown: "Nothing deleted",
		},
		{
			name:      "declined with n",
			answer:    "n\n",
			wantShown: "Nothing deleted",
		},
		{
			name:      "closed input",
			wantShown: "Nothing deleted",
		},
		{
			name:        "the server refuses while a fleet is owned",
			answer:      "y\n",
			refusal:     "you still own fleets, hand each one over or delete it first",
			refusalCode: http.StatusConflict,
			wantDeleted: true,
			wantError:   "you still own fleets, hand each one over or delete it first",
		},
		{
			name:      "arguments are refused",
			arguments: []string{"everything"},
			wantError: "takes no arguments",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deleted := false

			mux := http.NewServeMux()

			mux.HandleFunc("DELETE /account", func(w http.ResponseWriter, r *http.Request) {
				deleted = true

				if test.refusal != "" {
					http.Error(w, test.refusal, test.refusalCode)
					return
				}

				w.WriteHeader(http.StatusNoContent)
			})

			loggedInTestServer(t, mux)

			path, err := keyPath()

			if err != nil {
				t.Fatal(err)
			}

			answerOnStdin(t, test.answer)

			printed, err := captureStdout(t, func() error {
				return AccountDelete(test.arguments)
			})

			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error = %v, want it to mention %q", err, test.wantError)
				}
			} else if err != nil {
				t.Fatal(err)
			}

			if deleted != test.wantDeleted {
				t.Errorf("the server saw the account deleted = %v, want %v", deleted, test.wantDeleted)
			}

			if test.wantShown != "" && !strings.Contains(printed, test.wantShown) {
				t.Errorf("the output %q does not show %q", printed, test.wantShown)
			}

			// The stored login is worthless once the account is gone, and must
			// survive anything short of a completed delete
			_, statErr := os.Stat(path)

			switch {
			case test.wantDeleted && test.wantError == "" && statErr == nil:
				t.Error("the login is still stored although the account was deleted")

			case (!test.wantDeleted || test.wantError != "") && statErr != nil:
				t.Errorf("the login was removed although the account was not deleted: %v", statErr)
			}
		})
	}
}
