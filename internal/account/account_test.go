package account

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/siliconwitchery/superstack-cli/internal/api"
	"github.com/siliconwitchery/superstack-cli/internal/api/apitest"
)

func TestAccountBalance(t *testing.T) {
	tests := []struct {
		name       string
		arguments  []string
		fleets     string
		balances   string
		refusal    string
		wantLines  []string
		wantAbsent []string
		wantExact  string
		wantError  string
	}{
		{
			name:      "every fleet",
			arguments: []string{},
			fleets:    `[{"id":1,"name":"crew","owner":true},{"id":2,"name":"pilot","owner":false}]`,
			balances:  `[{"fleet":1,"balance":"15.000000","currency":"eur"},{"fleet":2,"balance":"0","currency":"eur"}]`,
			wantLines: []string{"ID", "NAME", "BALANCE", "crew", "€15.00", "pilot", "€0.00"},
		},
		{
			name:       "one fleet",
			arguments:  []string{"2"},
			fleets:     `[{"id":1,"name":"crew","owner":true},{"id":2,"name":"pilot","owner":false}]`,
			balances:   `[{"fleet":1,"balance":"15.000000","currency":"eur"},{"fleet":2,"balance":"0","currency":"eur"}]`,
			wantLines:  []string{"pilot", "€0.00"},
			wantAbsent: []string{"crew"},
		},
		{
			name:       "a fleet name with control characters is escaped",
			arguments:  []string{},
			fleets:     `[{"id":1,"name":"\u001b[2Kquiet","owner":true}]`,
			balances:   `[{"fleet":1,"balance":"15.000000","currency":"eur"}]`,
			wantLines:  []string{`\x1b[2Kquiet`},
			wantAbsent: []string{"\x1b"},
		},
		{
			name:      "machine readable",
			arguments: []string{"--json"},
			fleets:    `[{"id":1,"name":"crew","owner":true}]`,
			balances:  `[{"fleet":1,"balance":"15.000000","currency":"eur"}]`,
			wantExact: `[{"fleet":1,"balance":"15.000000","currency":"eur"}]` + "\n",
		},
		{
			name:      "machine readable for one fleet",
			arguments: []string{"2", "--json"},
			fleets:    `[{"id":1,"name":"crew","owner":true},{"id":2,"name":"pilot","owner":false}]`,
			balances:  `[{"fleet":1,"balance":"15.000000","currency":"eur"},{"fleet":2,"balance":"0","currency":"eur"}]`,
			wantExact: `[{"fleet":2,"balance":"0","currency":"eur"}]` + "\n",
		},
		{
			name:      "machine readable with no fleets",
			arguments: []string{"--json"},
			fleets:    `[]`,
			balances:  `[]`,
			wantExact: "[]\n",
		},
		{
			name:      "no fleets",
			arguments: []string{},
			fleets:    `[]`,
			balances:  `[]`,
			wantLines: []string{"No fleets yet"},
		},
		{
			name:      "a fleet without credit",
			arguments: []string{"1"},
			fleets:    `[{"id":1,"name":"crew","owner":true}]`,
			balances:  `[]`,
			wantExact: "No credit on that fleet yet.\n",
		},
		{
			name:      "a fleet the list does not name",
			arguments: []string{},
			fleets:    `[{"id":1,"name":"crew","owner":true}]`,
			balances:  `[{"fleet":99,"balance":"15.000000","currency":"eur"}]`,
			wantLines: []string{"99  -"},
		},
		{
			name:      "an unknown fleet",
			arguments: []string{"9"},
			fleets:    `[{"id":1,"name":"crew","owner":true}]`,
			balances:  `[]`,
			wantError: "no such fleet",
		},
		{
			name:      "server refusal",
			arguments: []string{},
			fleets:    `[{"id":1,"name":"crew","owner":true}]`,
			refusal:   "balances unavailable",
			wantError: "balances unavailable",
		},
		{
			name:      "a wordy id",
			arguments: []string{"crew"},
			wantError: "shown by fleet list",
		},
		{
			name:      "too many arguments",
			arguments: []string{"1", "2"},
			wantError: "at most one fleet id",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mux := http.NewServeMux()

			mux.HandleFunc("GET /fleets", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, test.fleets)
			})

			mux.HandleFunc("GET /balance", func(w http.ResponseWriter, r *http.Request) {
				if test.refusal != "" {
					http.Error(w, test.refusal, http.StatusServiceUnavailable)
					return
				}

				fmt.Fprint(w, test.balances)
			})

			session, out := apitest.LoggedInSession(t, mux)

			err := Balance(session, test.arguments)

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

			if test.wantExact != "" && printed != test.wantExact {
				t.Errorf("the output is %q, want exactly %q", printed, test.wantExact)
			}

			for _, want := range test.wantLines {
				if !strings.Contains(printed, want) {
					t.Errorf("the output %q does not show %q", printed, want)
				}
			}

			for _, absent := range test.wantAbsent {
				if strings.Contains(printed, absent) {
					t.Errorf("the output %q shows %q although it was filtered out", printed, absent)
				}
			}
		})
	}
}

func TestAccountTopup(t *testing.T) {
	tests := []struct {
		name        string
		arguments   []string
		stdin       string
		wantPath    string
		wantBrowser bool
		emptyBody   bool
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
			name:      "response has no url",
			arguments: []string{"3"},
			wantPath:  "/fleets/3/topup",
			emptyBody: true,
			wantError: "could not open the top-up page, try again",
		},
		{
			name:      "the server refuses",
			arguments: []string{"9"},
			wantPath:  "/fleets/9/topup",
			refusal:   "no such fleet",
			wantError: "no such fleet",
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

				if test.emptyBody {
					fmt.Fprint(w, `{}`)
					return
				}

				fmt.Fprint(w, `{"url":"https://checkout.stripe.com/c/pay/cs_test_1"}`)
			})

			session, out := apitest.LoggedInSession(t, mux)
			session.In = strings.NewReader(test.stdin)

			browserOpens := make(chan string, 1)
			session.OpenBrowser = func(url string) { browserOpens <- url }

			err := Topup(session, test.arguments)

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

			if !strings.Contains(printed, "The credit appears on the balance once the top-up completes.") {
				t.Errorf("the output %q does not explain when the top-up appears", printed)
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

			// The command asks the server whether the stored login still works
			// before it puts the question, so every case has to answer this.
			mux.HandleFunc("GET /fleets", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `[]`)
			})

			mux.HandleFunc("DELETE /account", func(w http.ResponseWriter, r *http.Request) {
				deleted = true

				if test.refusal != "" {
					http.Error(w, test.refusal, test.refusalCode)
					return
				}

				w.WriteHeader(http.StatusNoContent)
			})

			session, out := apitest.LoggedInSession(t, mux)

			path, err := api.KeyPath()

			if err != nil {
				t.Fatal(err)
			}

			session.In = strings.NewReader(test.answer)

			err = Delete(session, test.arguments)

			printed := out.String()

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

func TestAccountDeleteAsksNothingWhenTheServerIsGone(t *testing.T) {
	session, out := apitest.LoggedInSession(t, http.NewServeMux())

	gone := httptest.NewServer(http.NotFoundHandler())
	gone.Close()

	session.Base = gone.URL

	err := Delete(session, nil)

	if err == nil || !strings.Contains(err.Error(), "could not be reached") {
		t.Fatalf("error = %v, want it to mention the server could not be reached", err)
	}

	if out.String() != "" {
		t.Errorf("it asked %q before finding the server was gone", out.String())
	}
}

func TestAccountDeleteStopsWhenTheProbeIsRefused(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		wantError string
	}{
		{
			name:      "the login is no longer valid",
			status:    http.StatusUnauthorized,
			body:      "the login is no longer valid, log in again",
			wantError: "no longer valid",
		},
		{
			name:      "the server cannot answer",
			status:    http.StatusServiceUnavailable,
			body:      "the server could not list the fleets",
			wantError: "could not list the fleets",
		},
		{
			name:      "the version gate refuses",
			status:    http.StatusUpgradeRequired,
			body:      "update superstack to carry on",
			wantError: "update superstack",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deleted := false

			mux := http.NewServeMux()

			mux.HandleFunc("GET /fleets", func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, test.body, test.status)
			})

			mux.HandleFunc("DELETE /account", func(w http.ResponseWriter, r *http.Request) {
				deleted = true
			})

			session, out := apitest.LoggedInSession(t, mux)

			session.In = strings.NewReader("y\n")

			err := Delete(session, nil)

			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("error = %v, want the server's own refusal", err)
			}

			if out.String() != "" {
				t.Errorf("output = %q, want the question never to be put", out.String())
			}

			if deleted {
				t.Error("the server saw the account deleted although the probe was refused")
			}
		})
	}
}
