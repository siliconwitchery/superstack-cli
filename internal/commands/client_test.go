package commands

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func isolateKeyStorage(t *testing.T) string {
	t.Helper()

	temporary := t.TempDir()

	t.Setenv("HOME", temporary)
	t.Setenv("XDG_STATE_HOME", temporary)
	t.Setenv("AppData", temporary)

	return temporary
}

func loggedInSession(t *testing.T, handler http.Handler) (Session, *bytes.Buffer) {
	t.Helper()

	isolateKeyStorage(t)

	path, err := keyPath()

	if err != nil {
		t.Fatal(err)
	}

	err = os.MkdirAll(filepath.Dir(path), 0o700)

	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(path, []byte("ssk_test\n"), 0o600)

	if err != nil {
		t.Fatal(err)
	}

	authorized := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer ssk_test" {
			t.Errorf("%s %s carried authorization %q, want the stored key",
				r.Method, r.URL.Path, r.Header.Get("Authorization"))
		}

		handler.ServeHTTP(w, r)
	})

	server := httptest.NewServer(authorized)

	t.Cleanup(server.Close)

	out := &bytes.Buffer{}
	session := NewSession(server.URL, "test", strings.NewReader(""), out)
	session.OpenBrowser = func(url string) {}

	return session, out
}

func TestKeyPathStaysOutOfPublishedDotfiles(t *testing.T) {
	temporary := isolateKeyStorage(t)

	if runtime.GOOS != "linux" {
		path, err := keyPath()

		if err != nil {
			t.Fatal(err)
		}

		if !strings.HasPrefix(path, temporary) {
			t.Fatalf("keyPath() = %q, want it under the isolated home", path)
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

			path, err := keyPath()

			if err != nil {
				t.Fatal(err)
			}

			if path != test.wantPath {
				t.Errorf("keyPath() = %q, want %q", path, test.wantPath)
			}

			if strings.Contains(path, ".config") {
				t.Errorf("keyPath() = %q, must never sit in ~/.config", path)
			}
		})
	}
}

func TestTakeServerFlag(t *testing.T) {
	tests := []struct {
		name          string
		arguments     []string
		wantRemaining string
		wantBase      string
		wantError     string
	}{
		{
			name:          "no flag",
			arguments:     []string{"login"},
			wantRemaining: "login",
		},
		{
			name:          "a url",
			arguments:     []string{"--server", "http://localhost:8080", "login"},
			wantRemaining: "login",
			wantBase:      "http://localhost:8080",
		},
		{
			name:          "a url in equals form",
			arguments:     []string{"logout", "--server=https://staging.example.com"},
			wantRemaining: "logout",
			wantBase:      "https://staging.example.com",
		},
		{
			name:          "a trailing slash is trimmed",
			arguments:     []string{"--server", "http://localhost:8080/", "login"},
			wantRemaining: "login",
			wantBase:      "http://localhost:8080",
		},
		{
			name:          "the flag between command words",
			arguments:     []string{"fleet", "--server=http://localhost:9999", "list"},
			wantRemaining: "fleet list",
			wantBase:      "http://localhost:9999",
		},
		{
			name:      "a missing value",
			arguments: []string{"login", "--server"},
			wantError: "needs an address",
		},
		{
			name:      "an empty value",
			arguments: []string{"login", "--server="},
			wantError: "needs an address",
		},
		{
			name:      "an empty value from an unset shell variable",
			arguments: []string{"--server", "", "login"},
			wantError: "needs an address",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			remaining, base, err := TakeServerFlag(test.arguments)

			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error = %v, want it to mention %q", err, test.wantError)
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			if strings.Join(remaining, " ") != test.wantRemaining {
				t.Errorf("remaining = %q, want %q", strings.Join(remaining, " "), test.wantRemaining)
			}

			wantBase := test.wantBase

			if wantBase == "" {
				wantBase = defaultApiBase
			}

			if base != wantBase {
				t.Errorf("base = %q, want %q", base, wantBase)
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
			wantUrl: defaultApiBase + "/login",
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
				base = defaultApiBase
			}

			session := NewSession(base, "1.2.3", strings.NewReader(""), &bytes.Buffer{})

			request, err := apiRequest(session, http.MethodGet, "/login", nil)

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

func TestCheckServer(t *testing.T) {
	reachable := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(reachable.Close)

	unreachable := httptest.NewServer(http.NotFoundHandler())
	unreachable.Close()
	tests := []struct {
		name      string
		base      string
		wantError string
	}{
		{name: "reachable", base: reachable.URL},
		{name: "unreachable", base: unreachable.URL, wantError: "the server could not be reached, check your connection"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			session := NewSession(test.base, "test", strings.NewReader(""), &bytes.Buffer{})

			err := CheckServer(session)

			if test.wantError != "" {
				if err == nil || err.Error() != test.wantError {
					t.Fatalf("error = %v, want %q", err, test.wantError)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}
