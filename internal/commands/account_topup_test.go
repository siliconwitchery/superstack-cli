package commands

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestAccountTopup(t *testing.T) {
	tests := []struct {
		name        string
		arguments   []string
		stdin       string
		wantPath    string
		wantBrowser bool
		refusal     string
		wantError   string
	}{
		{
			name:        "a top-up link opened on enter",
			arguments:   []string{"3"},
			stdin:       "\n",
			wantPath:    "/fleets/3/topup",
			wantBrowser: true,
		},
		{
			name:      "a top-up link left alone",
			arguments: []string{"3"},
			wantPath:  "/fleets/3/topup",
		},
		{
			name:      "no fleet id",
			arguments: []string{},
			wantError: "takes a fleet id",
		},
		{
			name:      "too many arguments",
			arguments: []string{"3", "4"},
			wantError: "takes a fleet id",
		},
		{
			name:      "a wordy id",
			arguments: []string{"pilot"},
			wantError: "shown by fleet list",
		},
		{
			name:      "the server refuses",
			arguments: []string{"9"},
			wantPath:  "/fleets/9/topup",
			refusal:   "no such fleet",
			wantError: "the server said: no such fleet",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mux := http.NewServeMux()

			mux.HandleFunc("POST /fleets/{id}/topup", func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != test.wantPath {
					t.Errorf("the request went to %s, want %s", r.URL.Path, test.wantPath)
				}

				if test.refusal != "" {
					http.Error(w, test.refusal, http.StatusNotFound)
					return
				}

				fmt.Fprint(w, `{"url":"https://checkout.stripe.com/c/pay/cs_test_1"}`)
			})

			session, out := loggedInSession(t, mux)
			session.In = strings.NewReader(test.stdin)

			browserOpens := make(chan string, 1)
			session.OpenBrowser = func(url string) { browserOpens <- url }

			err := AccountTopup(session, test.arguments)

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

			if !strings.Contains(printed, "https://checkout.stripe.com/c/pay/cs_test_1") {
				t.Errorf("the output %q does not show the payment link", printed)
			}

			select {
			case url := <-browserOpens:
				if !test.wantBrowser {
					t.Errorf("the browser opened %q although enter was never pressed", url)
				} else if url != "https://checkout.stripe.com/c/pay/cs_test_1" {
					t.Errorf("the browser opened %q, want the payment link", url)
				}

			default:
				if test.wantBrowser {
					t.Error("the browser never opened")
				}
			}
		})
	}
}
