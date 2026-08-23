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

func IsolateLoginKeyStorage(t *testing.T) string {
	t.Helper()

	temporary := t.TempDir()

	t.Setenv("HOME", temporary)
	t.Setenv("XDG_STATE_HOME", temporary)
	t.Setenv("AppData", temporary)

	return temporary
}

func LoggedInInvocation(t *testing.T, handler http.Handler) (api.Invocation, *bytes.Buffer) {
	t.Helper()

	IsolateLoginKeyStorage(t)

	path, err := api.LoginKeyPath()

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
			t.Errorf("%s %s carried authorization %q, want the stored login key",
				r.Method, r.URL.Path, r.Header.Get("Authorization"))
		}

		handler.ServeHTTP(w, r)
	})

	server := httptest.NewServer(authorized)

	t.Cleanup(server.Close)

	out := &bytes.Buffer{}
	invocation := api.NewInvocation(server.URL, "test", strings.NewReader(""), out)
	invocation.OpenBrowser = func(url string) {}

	return invocation, out
}
