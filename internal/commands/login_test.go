package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func fakeProviderForLogin(t *testing.T, provider string, deviceInterval int, pollAnswers []string) *[]time.Time {
	t.Helper()

	devicePath := "/login/device/code"
	pollPath := "/login/oauth/access_token"
	wantClientId := "test-github-client"
	wantScope := "user:email"

	if provider == "gitlab" {
		devicePath = "/oauth/authorize_device"
		pollPath = "/oauth/token"
		wantClientId = "test-gitlab-client"
		wantScope = "read_user"
	}

	polledAt := &[]time.Time{}

	mux := http.NewServeMux()

	mux.HandleFunc("POST "+devicePath, func(w http.ResponseWriter, r *http.Request) {
		// Real github answers form-encoded without this header, so a client
		// that drops it must fail here too.
		if r.Header.Get("Accept") != "application/json" {
			t.Error("the device code request does not accept json")
			http.Error(w, "not acceptable", http.StatusNotAcceptable)
			return
		}

		r.ParseForm()

		if r.PostForm.Get("client_id") != wantClientId || r.PostForm.Get("scope") != wantScope {
			http.Error(w, "wrong form", http.StatusBadRequest)
			return
		}

		verificationUriComplete := ""

		if provider == "gitlab" {
			verificationUriComplete = `"verification_uri_complete": "https://gitlab.com/-/user_settings/device?user_code=WDJB-MJHT",`
		}

		fmt.Fprintf(w, `{
			"device_code": "test-device-code",
			"user_code": "WDJB-MJHT",
			"verification_uri": "https://example.com/device",
			%s
			"expires_in": 900,
			"interval": %d
		}`, verificationUriComplete, deviceInterval)
	})

	mux.HandleFunc("POST "+pollPath, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/json" {
			t.Error("the poll request does not accept json")
			http.Error(w, "not acceptable", http.StatusNotAcceptable)
			return
		}

		r.ParseForm()

		if r.PostForm.Get("client_id") != wantClientId ||
			r.PostForm.Get("device_code") != "test-device-code" ||
			r.PostForm.Get("grant_type") != "urn:ietf:params:oauth:grant-type:device_code" {
			http.Error(w, "wrong form", http.StatusBadRequest)
			return
		}

		if len(*polledAt) >= len(pollAnswers) {
			t.Error("polled more often than the script allows")
			http.Error(w, "over-polled", http.StatusTooManyRequests)
			return
		}

		*polledAt = append(*polledAt, time.Now())

		answer := pollAnswers[len(*polledAt)-1]

		// Gitlab carries rfc 8628 errors on a 400 status; github uses 200.
		if provider == "gitlab" && strings.Contains(answer, `"error"`) {
			w.WriteHeader(http.StatusBadRequest)
		}

		fmt.Fprint(w, answer)
	})

	server := httptest.NewServer(mux)

	t.Cleanup(server.Close)

	if provider == "gitlab" {
		previousBase := gitlabBase

		gitlabBase = server.URL

		t.Cleanup(func() { gitlabBase = previousBase })
	} else {
		previousBase := githubBase

		githubBase = server.URL

		t.Cleanup(func() { githubBase = previousBase })
	}

	return polledAt
}

func fakeSuperstack(t *testing.T) {
	t.Helper()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /login", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
			"github_client_id": "test-github-client",
			"gitlab_client_id": "test-gitlab-client"
		}`)
	})

	mux.HandleFunc("POST /login", func(w http.ResponseWriter, r *http.Request) {
		body := struct {
			Provider    string `json:"provider"`
			AccessToken string `json:"access_token"`
		}{}

		json.NewDecoder(r.Body).Decode(&body)

		githubPair := body.Provider == "github" && body.AccessToken == "gho_test"
		gitlabPair := body.Provider == "gitlab" && body.AccessToken == "glpat-test"

		if !githubPair && !gitlabPair {
			http.Error(w, "github did not confirm the login, try again", http.StatusUnauthorized)
			return
		}

		fmt.Fprint(w, `{"key": "ssk_test", "email": "someone@example.com"}`)
	})

	server := httptest.NewServer(mux)

	t.Cleanup(server.Close)

	chosenApiBase = server.URL

	t.Cleanup(func() { chosenApiBase = "" })
}

func TestLogin(t *testing.T) {
	tests := []struct {
		name           string
		provider       string
		deviceInterval int
		pollAnswers    []string
		wantError      string
		wantPollGap    time.Duration
	}{
		{
			name:        "github approved on the first poll",
			provider:    "github",
			pollAnswers: []string{`{"access_token": "gho_test"}`},
		},
		{
			name:        "gitlab approved on the first poll",
			provider:    "gitlab",
			pollAnswers: []string{`{"access_token": "glpat-test"}`},
		},
		{
			name:     "github approved after pending",
			provider: "github",
			pollAnswers: []string{
				`{"error": "authorization_pending"}`,
				`{"error": "authorization_pending"}`,
				`{"access_token": "gho_test"}`,
			},
		},
		{
			name:     "gitlab approved after pending",
			provider: "gitlab",
			pollAnswers: []string{
				`{"error": "authorization_pending"}`,
				`{"access_token": "glpat-test"}`,
			},
		},
		{
			name:     "slowed down then approved",
			provider: "github",
			pollAnswers: []string{
				`{"error": "slow_down", "interval": 1}`,
				`{"access_token": "gho_test"}`,
			},
			wantPollGap: time.Second,
		},
		{
			name:     "gitlab slowed down without an interval",
			provider: "gitlab",
			pollAnswers: []string{
				`{"error": "slow_down"}`,
				`{"access_token": "glpat-test"}`,
			},
			wantPollGap: 5 * time.Second,
		},
		{
			name:           "the device interval is honored",
			provider:       "github",
			deviceInterval: 1,
			pollAnswers: []string{
				`{"error": "authorization_pending"}`,
				`{"access_token": "gho_test"}`,
			},
			wantPollGap: time.Second,
		},
		{
			name:        "github declined",
			provider:    "github",
			pollAnswers: []string{`{"error": "access_denied"}`},
			wantError:   "declined",
		},
		{
			name:        "gitlab declined",
			provider:    "gitlab",
			pollAnswers: []string{`{"error": "access_denied"}`},
			wantError:   "declined",
		},
		{
			name:        "code expired",
			provider:    "github",
			pollAnswers: []string{`{"error": "expired_token"}`},
			wantError:   "expired",
		},
		{
			name:        "server rejects the token",
			provider:    "github",
			pollAnswers: []string{`{"access_token": "gho_stolen"}`},
			wantError:   "did not confirm the login",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isolateKeyStorage(t)

			answerOnStdin(t, "")

			captureBrowserOpens(t)

			previousMinimum := minimumPollInterval

			minimumPollInterval = 0

			t.Cleanup(func() { minimumPollInterval = previousMinimum })

			polledAt := fakeProviderForLogin(t, test.provider, test.deviceInterval, test.pollAnswers)

			fakeSuperstack(t)

			err := Login([]string{test.provider})

			if test.wantPollGap > 0 {
				if len(*polledAt) < 2 {
					t.Fatalf("only %d polls arrived, want at least 2", len(*polledAt))
				}

				gap := (*polledAt)[1].Sub((*polledAt)[0])

				if gap < test.wantPollGap {
					t.Errorf("the polls came %v apart, want at least %v", gap, test.wantPollGap)
				}
			}

			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error = %v, want it to mention %q", err, test.wantError)
				}

				path, _ := keyPath()

				if _, statError := os.Stat(path); statError == nil {
					t.Fatal("a key was stored despite the failed login")
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			path, err := keyPath()

			if err != nil {
				t.Fatal(err)
			}

			stored, err := os.ReadFile(path)

			if err != nil {
				t.Fatal(err)
			}

			if strings.TrimSpace(string(stored)) != "ssk_test" {
				t.Errorf("stored key = %q, want ssk_test", stored)
			}

			info, err := os.Stat(path)

			if err != nil {
				t.Fatal(err)
			}

			if info.Mode().Perm() != 0o600 {
				t.Errorf("key file mode = %v, want 0600", info.Mode().Perm())
			}
		})
	}
}

func TestLoginOpensTheBrowserOnEnter(t *testing.T) {
	isolateKeyStorage(t)

	previousMinimum := minimumPollInterval

	minimumPollInterval = 0

	t.Cleanup(func() { minimumPollInterval = previousMinimum })

	fakeProviderForLogin(t, "gitlab", 0, []string{`{"access_token": "glpat-test"}`})

	fakeSuperstack(t)

	browserOpens := captureBrowserOpens(t)

	answerOnStdin(t, "\n")

	_, err := captureStdout(t, func() error {
		return Login([]string{"gitlab"})
	})

	if err != nil {
		t.Fatal(err)
	}

	select {
	case url := <-browserOpens:
		if url != "https://gitlab.com/-/user_settings/device?user_code=WDJB-MJHT" {
			t.Errorf("the browser opened %q, want the verification link with the code filled in", url)
		}

	case <-time.After(2 * time.Second):
		t.Fatal("the browser never opened")
	}
}

func TestLoginRequiresAProvider(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
	}{
		{"no arguments", nil},
		{"an unknown provider", []string{"bitbucket"}},
		{"a misspelled provider", []string{"gitlabb"}},
		{"too many arguments", []string{"github", "extra"}},
	}

	for _, test := range tests {
		err := Login(test.arguments)

		if err == nil || !strings.Contains(err.Error(), "a provider: github or gitlab") {
			t.Errorf("%s: error = %v, want the provider hint", test.name, err)
		}
	}
}

func TestLoginProviderNotOffered(t *testing.T) {
	isolateKeyStorage(t)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /login", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"github_client_id": "test-github-client"}`)
	})

	server := httptest.NewServer(mux)

	t.Cleanup(server.Close)

	chosenApiBase = server.URL

	t.Cleanup(func() { chosenApiBase = "" })

	err := Login([]string{"gitlab"})

	if err == nil || !strings.Contains(err.Error(), "offers no gitlab login") {
		t.Fatalf("error = %v, want it to say the server offers no gitlab login", err)
	}
}
