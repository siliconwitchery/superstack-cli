package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func fakeProviderForLogin(t *testing.T, provider string, deviceInterval int, deviceAnswer string, pollAnswers []string) (*[]time.Time, string) {
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
		// Real github answers form-encoded unless this header is present.
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

		if deviceAnswer != "" {
			fmt.Fprint(w, deviceAnswer)
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

	return polledAt, server.URL
}

func fakeSuperstack(t *testing.T, providersRefusal string, loginAnswer string) (Session, *bytes.Buffer) {
	t.Helper()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /login", func(w http.ResponseWriter, r *http.Request) {
		if providersRefusal != "" {
			http.Error(w, providersRefusal, http.StatusServiceUnavailable)
			return
		}

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

		if loginAnswer != "" {
			fmt.Fprint(w, loginAnswer)
			return
		}

		fmt.Fprint(w, `{"key": "ssk_test", "email": "someone@example.com"}`)
	})

	server := httptest.NewServer(mux)

	t.Cleanup(server.Close)

	out := &bytes.Buffer{}
	session := NewSession(server.URL, "test", strings.NewReader(""), out)
	session.OpenBrowser = func(url string) {}

	return session, out
}

func TestLogin(t *testing.T) {
	tests := []struct {
		name           string
		provider       string
		deviceInterval int
		deviceAnswer   string
		pollAnswers    []string
		providersError string
		loginAnswer    string
		wantError      string
		wantPollGap    time.Duration
	}{
		{
			name:           "superstack refuses the provider list",
			provider:       "github",
			providersError: "login unavailable",
			wantError:      "login unavailable",
		},
		{
			name:         "provider refuses the device code",
			provider:     "github",
			deviceAnswer: `{"error":"access_denied"}`,
			wantError:    "github would not start the login, try again",
		},
		{
			name:         "deadline passes before approval",
			provider:     "github",
			deviceAnswer: `{"device_code":"test-device-code","user_code":"WDJB-MJHT","verification_uri":"https://example.com/device","expires_in":0,"interval":1}`,
			wantError:    "the code expired before it was entered, run login again",
		},
		{
			name:        "poll has neither an error nor a token",
			provider:    "github",
			pollAnswers: []string{`{}`},
			wantError:   "the login did not complete on github, run login again",
		},
		{
			name:        "poll has an unrecognised error",
			provider:    "github",
			pollAnswers: []string{`{"error":"server_error"}`},
			wantError:   "the login did not complete on github, run login again",
		},
		{
			name:        "superstack returns an empty key",
			provider:    "github",
			pollAnswers: []string{`{"access_token":"gho_test"}`},
			loginAnswer: `{"key":"","email":"someone@example.com"}`,
			wantError:   "the login did not complete",
		},
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
			wantPollGap: 6 * time.Second,
		},
		{
			name:     "gitlab slowed down without an interval",
			provider: "gitlab",
			pollAnswers: []string{
				`{"error": "slow_down"}`,
				`{"access_token": "glpat-test"}`,
			},
			wantPollGap: 6 * time.Second,
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

			deviceInterval := test.deviceInterval

			if deviceInterval == 0 {
				deviceInterval = 1
			}

			polledAt, providerBase := fakeProviderForLogin(t, test.provider, deviceInterval, test.deviceAnswer, test.pollAnswers)
			session, _ := fakeSuperstack(t, test.providersError, test.loginAnswer)

			if test.provider == "gitlab" {
				session.GitlabBase = providerBase
			} else {
				session.GithubBase = providerBase
			}

			err := Login(session, []string{test.provider})

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

	_, providerBase := fakeProviderForLogin(t, "gitlab", 1, "", []string{`{"access_token": "glpat-test"}`})
	session, _ := fakeSuperstack(t, "", "")
	session.GitlabBase = providerBase
	session.In = strings.NewReader("\n")
	browserOpens := make(chan string, 1)
	session.OpenBrowser = func(url string) { browserOpens <- url }

	err := Login(session, []string{"gitlab"})

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
		err := Login(Session{}, test.arguments)

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

	out := &bytes.Buffer{}
	session := NewSession(server.URL, "test", strings.NewReader(""), out)

	err := Login(session, []string{"gitlab"})

	if err == nil || !strings.Contains(err.Error(), "offers no gitlab login") {
		t.Fatalf("error = %v, want it to say the server offers no gitlab login", err)
	}
}
