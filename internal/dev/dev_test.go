package dev

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/siliconwitchery/superstack-cli/internal/api"
	"github.com/siliconwitchery/superstack-cli/internal/api/apitest"
)

// Pairs one device with fleet 3, accepts uploads to that device and fleet, serves two stored logs, and holds
// every follow request until released, when it refuses it as no such fleet so that dev ends
type testServer struct {
	mutex    sync.Mutex
	uploads  []string
	queries  []string
	released chan struct{}
}

func (server *testServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/devices":
		name := "boiler"
		json.NewEncoder(w).Encode([]api.DeviceEntry{{Imei: "354820091234567", Name: &name, FleetId: 3}})

	case r.Method == http.MethodPut && r.URL.Path == "/devices/354820091234567/files":
		body, _ := io.ReadAll(r.Body)

		server.mutex.Lock()
		server.uploads = append(server.uploads, string(body))
		server.mutex.Unlock()

		w.WriteHeader(http.StatusNoContent)

	case r.Method == http.MethodPut && r.URL.Path == "/fleets/3/files":
		body, _ := io.ReadAll(r.Body)

		server.mutex.Lock()
		server.uploads = append(server.uploads, string(body))
		server.mutex.Unlock()

		w.Write([]byte(`{"devices": 2}`))

	case r.Method == http.MethodGet && r.URL.Path == "/fleets/3/logs":
		server.mutex.Lock()
		server.queries = append(server.queries, r.URL.RawQuery)
		server.mutex.Unlock()

		if r.URL.Query().Has("after") {
			<-server.released
			http.Error(w, "no such fleet", http.StatusNotFound)
			return
		}

		name := "boiler"
		receivedAt := time.Date(2026, 10, 6, 13, 4, 5, 0, time.UTC)
		json.NewEncoder(w).Encode(api.LogsAnswer{Logs: []api.LogEntry{
			{Id: 1, Imei: "354820091234567", Name: &name, Kind: "print", Text: "log 1", ReceivedAt: receivedAt},
			{Id: 2, Imei: "354820091234567", Name: &name, Kind: "print", Text: "log 2", ReceivedAt: receivedAt},
		}, Next: 2})

	default:
		http.Error(w, "unexpected request", http.StatusNotFound)
	}
}

func (server *testServer) uploadCount() int {
	server.mutex.Lock()
	defer server.mutex.Unlock()

	return len(server.uploads)
}

type safeBuffer struct {
	mutex sync.Mutex
	text  strings.Builder
}

func (buffer *safeBuffer) Write(data []byte) (int, error) {
	buffer.mutex.Lock()
	defer buffer.mutex.Unlock()

	return buffer.text.Write(data)
}

func (buffer *safeBuffer) String() string {
	buffer.mutex.Lock()
	defer buffer.mutex.Unlock()

	return buffer.text.String()
}

func waitFor(t *testing.T, what string, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)

	for !condition() {
		if time.Now().After(deadline) {
			t.Fatalf("gave up waiting for %s", what)
		}

		time.Sleep(50 * time.Millisecond)
	}
}

func TestDevRefusesArguments(t *testing.T) {
	project := t.TempDir()

	if err := os.WriteFile(filepath.Join(project, "main.lua"), []byte("print(1)"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		arguments []string
		wantError string
	}{
		{[]string{}, "takes an IMEI or fleet id"},
		{[]string{"354820091234567"}, "takes an IMEI or fleet id"},
		{[]string{"354820099999999", project}, "device 354820099999999 is not paired with any of your fleets"},
		{[]string{"pilot", project}, "the first argument is the 15-digit IMEI"},
		{[]string{"3", filepath.Join(project, "missing.lua")}, "does not exist"},
	}

	for _, test := range tests {
		t.Run(strings.Join(test.arguments, " "), func(t *testing.T) {
			invocation, _ := apitest.LoggedInInvocation(t, &testServer{released: make(chan struct{})})

			err := Dev(invocation, test.arguments)

			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("got %v, want an error containing %q", err, test.wantError)
			}
		})
	}
}

func TestDevUploadsOnChangeAndFollowsTheLog(t *testing.T) {
	tests := []struct {
		name         string
		target       string
		watchFile    bool
		wantQuery    string
		wantUploaded string
		wantRefusal  string
	}{
		{
			name:         "a file to a device",
			target:       "354820091234567",
			watchFile:    true,
			wantQuery:    "after=2&imei=354820091234567",
			wantUploaded: "Uploaded 1 file to device 354820091234567. It arrives at its next check-in.\n",
			wantRefusal:  "does not exist",
		},
		{
			name:         "a directory to a fleet",
			target:       "3",
			wantQuery:    "after=2",
			wantUploaded: "Uploaded 1 file to 2 devices in fleet 3. It arrives at each device's next check-in.\n",
			wantRefusal:  "there are no files to upload",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project := t.TempDir()
			mainLua := filepath.Join(project, "main.lua")

			if err := os.WriteFile(mainLua, []byte("print(1)"), 0o644); err != nil {
				t.Fatal(err)
			}

			watched := project

			if test.watchFile {
				watched = mainLua
			}

			server := &testServer{released: make(chan struct{})}
			invocation, _ := apitest.LoggedInInvocation(t, server)
			out := &safeBuffer{}
			invocation.Out = out
			ended := make(chan error, 1)

			go func() {
				ended <- Dev(invocation, []string{test.target, watched})
			}()

			// The first upload, then an edit
			waitFor(t, "the first upload", func() bool { return server.uploadCount() == 1 })

			if err := os.WriteFile(mainLua, []byte("print(2) -- edited"), 0o644); err != nil {
				t.Fatal(err)
			}

			waitFor(t, "the upload after the edit", func() bool { return server.uploadCount() == 2 })

			// A deleted file is reported, and its return is uploaded
			if err := os.Remove(mainLua); err != nil {
				t.Fatal(err)
			}

			waitFor(t, "the refusal", func() bool { return strings.Contains(out.String(), test.wantRefusal) })

			if err := os.WriteFile(mainLua, []byte("print(3)"), 0o644); err != nil {
				t.Fatal(err)
			}

			waitFor(t, "the upload after the return", func() bool { return server.uploadCount() == 3 })

			close(server.released)

			err := <-ended

			if err == nil || !strings.Contains(err.Error(), "no such fleet") {
				t.Fatalf("dev ended with %v, want the log's refusal", err)
			}

			server.mutex.Lock()
			defer server.mutex.Unlock()

			if len(server.queries) < 2 || server.queries[1] != test.wantQuery {
				t.Errorf("log queries were %q, want the follow to ask %q", server.queries, test.wantQuery)
			}

			wantBodies := []string{
				`{"files":{"main.lua":"cHJpbnQoMSk="}}`,
				`{"files":{"main.lua":"cHJpbnQoMikgLS0gZWRpdGVk"}}`,
				`{"files":{"main.lua":"cHJpbnQoMyk="}}`,
			}

			for index, body := range server.uploads {
				if body != wantBodies[index] {
					t.Errorf("upload %d sent %s, want %s", index+1, body, wantBodies[index])
				}
			}

			output := out.String()

			if strings.Count(output, test.wantUploaded) != 3 {
				t.Errorf("output was:\n%s\nwant %q three times", output, test.wantUploaded)
			}

			if !strings.Contains(output, "[boiler] log 1\n") || !strings.Contains(output, "[boiler] log 2\n") {
				t.Errorf("output was:\n%s\nwant the two stored logs", output)
			}

			if !strings.HasPrefix(output, test.wantUploaded) {
				t.Errorf("output was:\n%s\nwant the first upload before any log", output)
			}
		})
	}
}
