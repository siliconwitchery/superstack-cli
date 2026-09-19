package files

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/siliconwitchery/superstack-cli/internal/api"
	"github.com/siliconwitchery/superstack-cli/internal/api/apitest"
)

func writeTestProject(t *testing.T) string {
	t.Helper()

	project := t.TempDir()

	files := map[string]string{
		"main.lua":            "print(1)",
		"lib/sensor.lua":      "return 2",
		"lib/.hidden.lua":     "return 3",
		".git/HEAD":           "ref: refs/heads/main",
		"notes/.DS_Store":     "junk",
		"notes/threshold.txt": "3",
	}

	for name, content := range files {
		path := filepath.Join(project, filepath.FromSlash(name))

		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	return project
}

func TestUpload(t *testing.T) {
	project := writeTestProject(t)
	extra := filepath.Join(t.TempDir(), "config.txt")

	if err := os.WriteFile(extra, []byte("interval=5"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		arguments  []string
		wantPath   string
		wantFiles  map[string]string
		wantOutput string
	}{
		{
			name:      "a directory and a file to a device",
			arguments: []string{"354820091234567", project, extra},
			wantPath:  "/devices/354820091234567/files",
			wantFiles: map[string]string{
				"main.lua": "print(1)", "lib/sensor.lua": "return 2", "notes/threshold.txt": "3", "config.txt": "interval=5",
			},
			wantOutput: "Uploaded 4 files to device 354820091234567. They arrive at its next check-in.\n",
		},
		{
			name:       "one file to a fleet",
			arguments:  []string{"3", filepath.Join(project, "main.lua")},
			wantPath:   "/fleets/3/files",
			wantFiles:  map[string]string{"main.lua": "print(1)"},
			wantOutput: "Uploaded 1 file to 2 devices in fleet 3. They arrive at each device's next check-in.\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotPath := ""
			gotFiles := map[string]string{}
			mux := http.NewServeMux()
			mux.HandleFunc("PUT /", func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path

				if r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("content type = %q, want application/json", r.Header.Get("Content-Type"))
				}

				decoded := struct {
					Files map[string][]byte `json:"files"`
				}{}

				if err := json.NewDecoder(r.Body).Decode(&decoded); err != nil {
					t.Fatal(err)
				}

				for path, content := range decoded.Files {
					gotFiles[path] = string(content)
				}

				if strings.HasPrefix(r.URL.Path, "/fleets/") {
					w.Write([]byte(`{"devices":2}`))
					return
				}

				w.WriteHeader(http.StatusNoContent)
			})

			invocation, out := apitest.LoggedInInvocation(t, mux)

			err := Upload(invocation, test.arguments)

			if err != nil {
				t.Fatal(err)
			}

			if gotPath != test.wantPath {
				t.Errorf("request path = %q, want %q", gotPath, test.wantPath)
			}

			if !reflect.DeepEqual(gotFiles, test.wantFiles) {
				t.Errorf("uploaded files = %v, want %v", gotFiles, test.wantFiles)
			}

			if out.String() != test.wantOutput {
				t.Errorf("output = %q, want %q", out.String(), test.wantOutput)
			}
		})
	}
}

func TestUploadArgumentsAndRefusal(t *testing.T) {
	project := writeTestProject(t)
	empty := t.TempDir()

	tests := []struct {
		name      string
		arguments []string
		wantError string
	}{
		{"no arguments", nil, "takes an IMEI or fleet id"},
		{"no files", []string{"354820091234567"}, "takes an IMEI or fleet id"},
		{"wordy target", []string{"rooftop", project}, "15-digit IMEI"},
		{"zero fleet id", []string{"0", project}, "15-digit IMEI"},
		{"missing file", []string{"3", filepath.Join(project, "absent.lua")}, "does not exist"},
		{"the same path twice", []string{"3", project, filepath.Join(project, "main.lua")}, "uploaded twice"},
		{"an empty directory", []string{"3", empty}, "no files to upload"},
	}

	for _, test := range tests {
		err := Upload(api.Invocation{}, test.arguments)

		if err == nil || !strings.Contains(err.Error(), test.wantError) {
			t.Errorf("%s: error = %v, want it to mention %q", test.name, err, test.wantError)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("PUT /devices/354820091234567/files", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "the upload must include main.lua", http.StatusBadRequest)
	})

	invocation, out := apitest.LoggedInInvocation(t, mux)

	err := Upload(invocation, []string{"354820091234567", filepath.Join(project, "lib")})

	if err == nil || err.Error() != "the upload must include main.lua" {
		t.Errorf("error = %v, want the server's refusal", err)
	}

	if out.String() != "" {
		t.Errorf("output = %q, want nothing", out.String())
	}
}
