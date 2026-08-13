package commands

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func captureStdout(t *testing.T, run func() error) (string, error) {
	t.Helper()

	readEnd, writeEnd, err := os.Pipe()

	if err != nil {
		t.Fatal(err)
	}

	stdout := os.Stdout

	os.Stdout = writeEnd

	runError := run()

	os.Stdout = stdout

	writeEnd.Close()

	printed, err := io.ReadAll(readEnd)

	if err != nil {
		t.Fatal(err)
	}

	return string(printed), runError
}

func isolateKeyStorage(t *testing.T) string {
	t.Helper()

	temporary := t.TempDir()

	t.Setenv("HOME", temporary)
	t.Setenv("XDG_STATE_HOME", temporary)
	t.Setenv("AppData", temporary)

	return temporary
}

func loggedInTestServer(t *testing.T, handler http.Handler) {
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

	// Every command reaching a logged-in server must carry the stored key, so
	// the fixture proves it once rather than each command remembering to
	authorized := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer ssk_test" {
			t.Errorf("%s %s carried authorization %q, want the stored key",
				r.Method, r.URL.Path, r.Header.Get("Authorization"))
		}

		handler.ServeHTTP(w, r)
	})

	server := httptest.NewServer(authorized)

	t.Cleanup(server.Close)

	chosenApiBase = server.URL

	t.Cleanup(func() { chosenApiBase = "" })
}

func TestKeyPathStaysOutOfPublishedDotfiles(t *testing.T) {
	temporary := isolateKeyStorage(t)

	path, err := keyPath()

	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(path, temporary) {
		t.Fatalf("keyPath() = %q, want it under the isolated home", path)
	}

	if runtime.GOOS != "linux" {
		return
	}

	if path != filepath.Join(temporary, "superstack", "key") {
		t.Errorf("keyPath() = %q, want it directly under XDG_STATE_HOME", path)
	}

	t.Setenv("XDG_STATE_HOME", "")

	path, err = keyPath()

	if err != nil {
		t.Fatal(err)
	}

	if path != filepath.Join(temporary, ".local", "state", "superstack", "key") {
		t.Errorf("keyPath() = %q, want the ~/.local/state fallback", path)
	}

	if strings.Contains(path, ".config") {
		t.Errorf("keyPath() = %q, must never sit in ~/.config: dotfile repos publish it", path)
	}

	t.Setenv("XDG_STATE_HOME", ".state")

	path, err = keyPath()

	if err != nil {
		t.Fatal(err)
	}

	if path != filepath.Join(temporary, ".local", "state", "superstack", "key") {
		t.Errorf("keyPath() = %q: a relative XDG_STATE_HOME must be ignored, never joined to the working directory", path)
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
			wantError: "needs a url",
		},
		{
			name:      "an empty value",
			arguments: []string{"login", "--server="},
			wantError: "needs a url",
		},
		{
			name:      "an empty value from an unset shell variable",
			arguments: []string{"--server", "", "login"},
			wantError: "needs a url",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			chosenApiBase = ""

			t.Cleanup(func() { chosenApiBase = "" })

			remaining, err := TakeServerFlag(test.arguments)

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

			if chosenApiBase != test.wantBase {
				t.Errorf("chosenApiBase = %q, want %q", chosenApiBase, test.wantBase)
			}
		})
	}
}

func TestApiRequestBase(t *testing.T) {
	previousVersion := CliVersion

	CliVersion = "1.2.3"

	t.Cleanup(func() { CliVersion = previousVersion })

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
			chosenApiBase = test.chosenBase

			t.Cleanup(func() { chosenApiBase = "" })

			request, err := apiRequest(http.MethodGet, "/login", nil)

			if err != nil {
				t.Fatal(err)
			}

			if request.URL.String() != test.wantUrl {
				t.Errorf("url = %q, want %q", request.URL.String(), test.wantUrl)
			}

			// Pinned to a literal, not to CliVersion: the server's gate parses
			// this exact shape, so deriving it here would agree with any value
			if request.Header.Get("User-Agent") != "superstack/1.2.3" {
				t.Errorf("User-Agent = %q, want superstack/1.2.3", request.Header.Get("User-Agent"))
			}
		})
	}
}

func TestCheckServer(t *testing.T) {
	reachable := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	defer reachable.Close()

	chosenApiBase = reachable.URL

	t.Cleanup(func() { chosenApiBase = "" })

	err := CheckServer()

	if err != nil {
		t.Fatalf("a reachable server reported: %v", err)
	}

	unreachable := httptest.NewServer(http.NotFoundHandler())

	unreachable.Close()

	chosenApiBase = unreachable.URL

	err = CheckServer()

	if err == nil || !strings.Contains(err.Error(), "cannot be reached") {
		t.Fatalf("error = %v, want the consistent cannot-be-reached message", err)
	}

	if !strings.Contains(err.Error(), unreachable.URL) {
		t.Errorf("error = %v, want it to name the server address", err)
	}
}
