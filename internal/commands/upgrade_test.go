package commands

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func fakeExecutable(t *testing.T, path string) {
	t.Helper()

	previous := currentExecutable

	currentExecutable = func() (string, error) { return path, nil }

	t.Cleanup(func() { currentExecutable = previous })
}

func fakeCliVersion(t *testing.T, version string) {
	t.Helper()

	previous := CliVersion

	CliVersion = version

	t.Cleanup(func() { CliVersion = previous })
}

func fakeRelease(t *testing.T, version string, binaryContent string, tampered bool) {
	t.Helper()

	archiveName := fmt.Sprintf("superstack_%s_%s_%s.tar.gz", version, runtime.GOOS, runtime.GOARCH)

	var archive bytes.Buffer

	gzipWriter := gzip.NewWriter(&archive)

	tarWriter := tar.NewWriter(gzipWriter)

	err := tarWriter.WriteHeader(&tar.Header{Name: "superstack", Mode: 0o755, Size: int64(len(binaryContent))})

	if err != nil {
		t.Fatal(err)
	}

	_, err = tarWriter.Write([]byte(binaryContent))

	if err != nil {
		t.Fatal(err)
	}

	tarWriter.Close()
	gzipWriter.Close()

	hash := sha256.Sum256(archive.Bytes())

	if tampered {
		hash = sha256.Sum256([]byte("something else entirely"))
	}

	mux := http.NewServeMux()

	server := httptest.NewServer(mux)

	t.Cleanup(server.Close)

	mux.HandleFunc("GET /siliconwitchery/superstack-cli/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, server.URL+"/siliconwitchery/superstack-cli/releases/tag/v"+version, http.StatusFound)
	})

	mux.HandleFunc("GET /siliconwitchery/superstack-cli/releases/download/v"+version+"/"+archiveName,
		func(w http.ResponseWriter, r *http.Request) {
			w.Write(archive.Bytes())
		})

	mux.HandleFunc("GET /siliconwitchery/superstack-cli/releases/download/v"+version+"/checksums.txt",
		func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(hash[:]), archiveName)
		})

	previousBase := githubBase

	githubBase = server.URL

	t.Cleanup(func() { githubBase = previousBase })
}

func TestUpgrade(t *testing.T) {
	directory := t.TempDir()

	executable := filepath.Join(directory, "superstack")

	err := os.WriteFile(executable, []byte("the old binary"), 0o755)

	if err != nil {
		t.Fatal(err)
	}

	fakeExecutable(t, executable)

	fakeCliVersion(t, "1.0.0")

	fakeRelease(t, "9.9.9", "the new binary", false)

	err = Upgrade(nil)

	if err != nil {
		t.Fatal(err)
	}

	replaced, err := os.ReadFile(executable)

	if err != nil {
		t.Fatal(err)
	}

	if string(replaced) != "the new binary" {
		t.Errorf("the binary now holds %q, want the new release", replaced)
	}

	info, err := os.Stat(executable)

	if err != nil {
		t.Fatal(err)
	}

	if info.Mode().Perm() != 0o755 {
		t.Errorf("the binary's mode is %v, want 0755", info.Mode().Perm())
	}

	if _, statError := os.Stat(executable + ".old"); !os.IsNotExist(statError) {
		t.Error("the renamed-aside binary was left behind")
	}
}

func TestUpgradeAlreadyLatest(t *testing.T) {
	directory := t.TempDir()

	executable := filepath.Join(directory, "superstack")

	err := os.WriteFile(executable, []byte("the current binary"), 0o755)

	if err != nil {
		t.Fatal(err)
	}

	fakeExecutable(t, executable)

	fakeCliVersion(t, "9.9.9")

	fakeRelease(t, "9.9.9", "the same binary", false)

	err = Upgrade(nil)

	if err != nil {
		t.Fatal(err)
	}

	kept, err := os.ReadFile(executable)

	if err != nil {
		t.Fatal(err)
	}

	if string(kept) != "the current binary" {
		t.Errorf("the binary now holds %q, want it untouched", kept)
	}
}

func TestUpgradeChecksumMismatch(t *testing.T) {
	directory := t.TempDir()

	executable := filepath.Join(directory, "superstack")

	err := os.WriteFile(executable, []byte("the old binary"), 0o755)

	if err != nil {
		t.Fatal(err)
	}

	fakeExecutable(t, executable)

	fakeCliVersion(t, "1.0.0")

	fakeRelease(t, "9.9.9", "the new binary", true)

	err = Upgrade(nil)

	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("error = %v, want the checksum refusal", err)
	}

	kept, err := os.ReadFile(executable)

	if err != nil {
		t.Fatal(err)
	}

	if string(kept) != "the old binary" {
		t.Errorf("the binary now holds %q, want it untouched after the failed checksum", kept)
	}
}

func TestUpgradeManagedInstalls(t *testing.T) {
	tests := []struct {
		name      string
		pathParts []string
		wantHint  string
	}{
		{"nix", []string{"nix", "store", "abc123-superstack", "bin", "superstack"}, "nix"},
		{"homebrew cellar", []string{"Cellar", "superstack", "1.0.0", "bin", "superstack"}, "homebrew"},
		{"homebrew caskroom", []string{"Caskroom", "superstack", "1.0.0", "superstack"}, "homebrew"},
		{"scoop", []string{"scoop", "apps", "superstack", "current", "superstack.exe"}, "scoop"},
	}

	unreachable := httptest.NewServer(http.NotFoundHandler())

	unreachable.Close()

	previousBase := githubBase

	githubBase = unreachable.URL

	t.Cleanup(func() { githubBase = previousBase })

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()

			path := filepath.Join(append([]string{root}, test.pathParts...)...)

			err := os.MkdirAll(filepath.Dir(path), 0o755)

			if err != nil {
				t.Fatal(err)
			}

			err = os.WriteFile(path, []byte("managed"), 0o755)

			if err != nil {
				t.Fatal(err)
			}

			fakeExecutable(t, path)

			err = Upgrade(nil)

			if err == nil || !strings.Contains(err.Error(), test.wantHint) {
				t.Fatalf("error = %v, want it to point at %s", err, test.wantHint)
			}
		})
	}
}

func TestUpgradeTakesNoArguments(t *testing.T) {
	err := Upgrade([]string{"now"})

	if err == nil || !strings.Contains(err.Error(), "takes no arguments") {
		t.Fatalf("error = %v, want the no-arguments hint", err)
	}
}
