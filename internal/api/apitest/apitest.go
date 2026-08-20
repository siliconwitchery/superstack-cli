package apitest

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/siliconwitchery/superstack-cli/internal/api"
)

func IsolateKeyStorage(t *testing.T) string {
	t.Helper()

	temporary := t.TempDir()

	t.Setenv("HOME", temporary)
	t.Setenv("XDG_STATE_HOME", temporary)
	t.Setenv("AppData", temporary)

	return temporary
}

func LoggedInSession(t *testing.T, handler http.Handler) (api.Session, *bytes.Buffer) {
	t.Helper()

	IsolateKeyStorage(t)

	path, err := api.KeyPath()

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
	session := api.NewSession(server.URL, "test", strings.NewReader(""), out)
	session.OpenBrowser = func(url string) {}

	return session, out
}
