package api_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/siliconwitchery/superstack-cli/internal/api"
	"github.com/siliconwitchery/superstack-cli/internal/api/apitest"
)

func TestKeyPathStaysOutOfPublishedDotfiles(t *testing.T) {
	temporary := apitest.IsolateLoginKeyStorage(t)

	if runtime.GOOS != "linux" {
		path, err := api.LoginKeyPath()

		if err != nil {
			t.Fatal(err)
		}

		if !strings.HasPrefix(path, temporary) {
			t.Fatalf("api.LoginKeyPath() = %q, want it under the isolated home", path)
		}

		return
	}

	tests := []struct {
		name      string
		stateHome string
		wantPath  string
	}{
		{name: "absolute state home", stateHome: temporary, wantPath: filepath.Join(temporary, "superstack", "key")},
		{name: "empty state home", stateHome: "", wantPath: filepath.Join(temporary, ".local", "state", "superstack", "key")},
		{name: "relative state home", stateHome: ".state", wantPath: filepath.Join(temporary, ".local", "state", "superstack", "key")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("XDG_STATE_HOME", test.stateHome)

			path, err := api.LoginKeyPath()

			if err != nil {
				t.Fatal(err)
			}

			if path != test.wantPath {
				t.Errorf("api.LoginKeyPath() = %q, want %q", path, test.wantPath)
			}

			if strings.Contains(path, ".config") {
				t.Errorf("api.LoginKeyPath() = %q, must never sit in ~/.config", path)
			}
		})
	}
}

func TestApiRequestBase(t *testing.T) {
	tests := []struct {
		name       string
		chosenBase string
		wantUrl    string
	}{
		{
			name:    "the default",
			wantUrl: api.DefaultBase + "/login",
		},
		{
			name:       "the flag overrides the default",
			chosenBase: "http://localhost:8888",
			wantUrl:    "http://localhost:8888/login",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			base := test.chosenBase

			if base == "" {
				base = api.DefaultBase
			}

			invocation := api.NewInvocation(base, "1.2.3", strings.NewReader(""), &bytes.Buffer{})

			request, err := api.Request(invocation, http.MethodGet, "/login", nil)

			if err != nil {
				t.Fatal(err)
			}

			if request.URL.String() != test.wantUrl {
				t.Errorf("url = %q, want %q", request.URL.String(), test.wantUrl)
			}

			// The server's version gate parses this exact User-Agent shape.
			if request.Header.Get("User-Agent") != "superstack/1.2.3" {
				t.Errorf("User-Agent = %q, want superstack/1.2.3", request.Header.Get("User-Agent"))
			}
		})
	}
}

func TestFetchFleetsFailures(t *testing.T) {
	tests := []struct {
		name           string
		loggedIn       bool
		storedLoginKey string
		status         int
		body           string
		wantError      string
	}{
		{name: "not logged in", wantError: "not logged in"},
		{name: "empty login key file", loggedIn: true, storedLoginKey: "  \n", wantError: "not logged in"},
		{name: "server refusal", loggedIn: true, status: http.StatusServiceUnavailable, body: "fleets unavailable", wantError: "fleets unavailable"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			apitest.IsolateLoginKeyStorage(t)

			invocation := api.Invocation{}

			if test.loggedIn {
				mux := http.NewServeMux()
				mux.HandleFunc("GET /fleets", func(w http.ResponseWriter, r *http.Request) {
					if test.status != 0 {
						w.WriteHeader(test.status)
					}

					fmt.Fprint(w, test.body)
				})

				invocation, _ = apitest.LoggedInInvocation(t, mux)

				if test.storedLoginKey != "" {
					path, err := api.LoginKeyPath()

					if err != nil {
						t.Fatal(err)
					}

					err = os.WriteFile(path, []byte(test.storedLoginKey), 0o600)

					if err != nil {
						t.Fatal(err)
					}
				}
			}

			_, err := api.FetchFleets(invocation)

			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("error = %v, want it to mention %q", err, test.wantError)
			}
		})
	}
}

func TestFetchDevicesFailures(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		wantError string
	}{
		{name: "server refusal", status: http.StatusServiceUnavailable, body: "devices unavailable", wantError: "devices unavailable"},
		{name: "undecodable body", status: http.StatusOK, body: `[{`, wantError: "could not be read"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("GET /devices", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.status)
				fmt.Fprint(w, test.body)
			})

			invocation, _ := apitest.LoggedInInvocation(t, mux)

			_, err := api.FetchDevices(invocation)

			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("error = %v, want it to mention %q", err, test.wantError)
			}
		})
	}
}

func TestFetchFleetKeysFailures(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		wantError string
	}{
		{name: "server refusal", status: http.StatusServiceUnavailable, body: "fleet keys unavailable", wantError: "fleet keys unavailable"},
		{name: "undecodable body", status: http.StatusOK, body: `{`, wantError: "could not be read"},
		{name: "a refusal carrying control characters", status: http.StatusServiceUnavailable, body: "\x1b[2Kgone", wantError: `\x1b[2Kgone`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("GET /fleet-keys", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.status)
				fmt.Fprint(w, test.body)
			})

			invocation, _ := apitest.LoggedInInvocation(t, mux)

			_, err := api.FetchFleetKeys(invocation)

			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("error = %v, want it to mention %q", err, test.wantError)
			}
		})
	}
}

func TestFetchBalancesFailures(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		wantError string
	}{
		{name: "server refusal", status: http.StatusServiceUnavailable, body: "balances unavailable", wantError: "balances unavailable"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("GET /balance", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.status)
				fmt.Fprint(w, test.body)
			})

			invocation, _ := apitest.LoggedInInvocation(t, mux)

			_, err := api.FetchBalances(invocation)

			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("error = %v, want it to mention %q", err, test.wantError)
			}
		})
	}
}

func TestFormatBalance(t *testing.T) {
	tests := []struct {
		balance   string
		want      string
		wantValue float64
		wantValid bool
	}{
		{balance: "15.000000", want: "\u20ac15.00", wantValue: 15, wantValid: true},
		{balance: "0", want: "\u20ac0.00", wantValue: 0, wantValid: true},
		{balance: "0.004", want: "\u20ac0.00", wantValue: 0.004, wantValid: true},
		{balance: "-2.50", want: "\u20ac-2.50", wantValue: -2.5, wantValid: true},
		{balance: "", want: "", wantValue: 0, wantValid: false},
		{balance: "not a number", want: "not a number", wantValue: 0, wantValid: false},
	}

	for _, test := range tests {
		t.Run(test.balance, func(t *testing.T) {
			formatted, value, valid := api.FormatBalance(api.BalanceEntry{Balance: test.balance})

			if formatted != test.want || value != test.wantValue || valid != test.wantValid {
				t.Errorf("api.FormatBalance() = %q, %v, %v, want %q, %v, %v",
					formatted, value, valid, test.want, test.wantValue, test.wantValid)
			}
		})
	}
}

func TestDecode(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantId    int64
		wantError string
	}{
		{name: "a whole object", body: `{"id":7}`, wantId: 7},
		{name: "a truncated object", body: `{"id":`, wantError: "could not be read"},
		{name: "a page from something that is not the server", body: "<html>bad gateway</html>", wantError: "could not be read"},
		{name: "nothing at all", body: "", wantError: "could not be read"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := &http.Response{Body: io.NopCloser(strings.NewReader(test.body))}

			value := struct {
				Id int64 `json:"id"`
			}{}

			err := api.Decode(response, &value)

			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error = %v, want it to mention %q", err, test.wantError)
				}

				for _, machinery := range []string{"unexpected EOF", "invalid character", "json", "EOF"} {
					if strings.Contains(err.Error(), machinery) {
						t.Errorf("the error %q shows the user %q", err, machinery)
					}
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			if value.Id != test.wantId {
				t.Errorf("id = %d, want %d", value.Id, test.wantId)
			}
		})
	}
}

type endlessBody struct{}

func (endlessBody) Read(destination []byte) (int, error) {
	for index := range destination {
		destination[index] = 'x'
	}

	return len(destination), nil
}

func TestDecodeStopsReadingABodyThatNeverEnds(t *testing.T) {
	response := &http.Response{
		Body: io.NopCloser(io.MultiReader(strings.NewReader(`{"name":"`), endlessBody{})),
	}

	value := struct {
		Name string `json:"name"`
	}{}

	finished := make(chan error, 1)

	go func() {
		finished <- api.Decode(response, &value)
	}()

	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("Decode accepted a body that never ends")
		}

	case <-time.After(30 * time.Second):
		t.Fatal("Decode is still reading a body that never ends")
	}
}
