package commands

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var currentExecutable = os.Executable

var downloadClient = &http.Client{Timeout: 5 * time.Minute}

func Upgrade(arguments []string) error {

	if len(arguments) != 0 {
		return errors.New("upgrade takes no arguments")
	}

	executable, err := currentExecutable()

	if err != nil {
		return err
	}

	executable, err = filepath.EvalSymlinks(executable)

	if err != nil {
		return err
	}

	// A package manager owns what it installed, and the nix store is
	// read-only, so upgrade replaces only binaries it owns itself
	switch {
	case strings.Contains(executable, "/nix/store/"):
		return errors.New("this install came from nix, update it through nix instead")

	case strings.Contains(executable, "/Cellar/") || strings.Contains(executable, "/Caskroom/"):
		return errors.New("this install came from homebrew, update it with: brew upgrade superstack")

	case strings.Contains(strings.ToLower(executable), filepath.Join("scoop", "apps")):
		return errors.New("this install came from scoop, update it with: scoop update superstack")

	case executable == "/usr/bin/superstack":
		return errors.New("this install came from your system's package manager, update it there instead")
	}

	// The newest release is wherever github's latest redirect lands, the
	// same trick install.sh uses, so no API and no rate limit
	latestRequest, err := http.NewRequest(http.MethodHead,
		githubBase+"/siliconwitchery/superstack-cli/releases/latest", nil)

	if err != nil {
		return err
	}

	latestResponse, err := downloadClient.Do(latestRequest)

	if err != nil {
		return fmt.Errorf("github could not be reached: %w", err)
	}

	latestResponse.Body.Close()

	landed := latestResponse.Request.URL.Path

	tag := ""

	if index := strings.LastIndex(landed, "/releases/tag/"); index >= 0 {
		tag = landed[index+len("/releases/tag/"):]
	}

	if tag == "" {
		return errors.New("no published release was found")
	}

	latestVersion := strings.TrimPrefix(tag, "v")

	if latestVersion == CliVersion {
		fmt.Println("You already have the latest release.")
		return nil
	}

	archiveName := fmt.Sprintf("superstack_%s_%s_%s.tar.gz", latestVersion, runtime.GOOS, runtime.GOARCH)

	binaryName := "superstack"

	if runtime.GOOS == "windows" {
		archiveName = fmt.Sprintf("superstack_%s_windows_%s.zip", latestVersion, runtime.GOARCH)

		binaryName = "superstack.exe"
	}

	downloadBase := githubBase + "/siliconwitchery/superstack-cli/releases/download/" + tag

	archiveResponse, err := downloadClient.Get(downloadBase + "/" + archiveName)

	if err != nil {
		return fmt.Errorf("github could not be reached: %w", err)
	}

	defer archiveResponse.Body.Close()

	if archiveResponse.StatusCode != http.StatusOK {
		return errors.New("the release has no download for this computer")
	}

	archiveBytes, err := io.ReadAll(archiveResponse.Body)

	if err != nil {
		return err
	}

	checksumsResponse, err := downloadClient.Get(downloadBase + "/checksums.txt")

	if err != nil {
		return fmt.Errorf("github could not be reached: %w", err)
	}

	defer checksumsResponse.Body.Close()

	if checksumsResponse.StatusCode != http.StatusOK {
		return errors.New("the release is missing its checksums")
	}

	checksums, err := io.ReadAll(io.LimitReader(checksumsResponse.Body, 1<<20))

	if err != nil {
		return err
	}

	archiveHash := sha256.Sum256(archiveBytes)

	wantHash := hex.EncodeToString(archiveHash[:])

	verified := false

	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)

		if len(fields) == 2 && fields[1] == archiveName && fields[0] == wantHash {
			verified = true
		}
	}

	if !verified {
		return errors.New("the download did not match the release's checksum, try again")
	}

	// Pull the binary out of the archive
	binaryBytes := []byte(nil)

	if runtime.GOOS == "windows" {
		zipReader, err := zip.NewReader(bytes.NewReader(archiveBytes), int64(len(archiveBytes)))

		if err != nil {
			return err
		}

		for _, file := range zipReader.File {
			if file.Name != binaryName {
				continue
			}

			opened, err := file.Open()

			if err != nil {
				return err
			}

			binaryBytes, err = io.ReadAll(opened)

			opened.Close()

			if err != nil {
				return err
			}
		}
	} else {
		gzipReader, err := gzip.NewReader(bytes.NewReader(archiveBytes))

		if err != nil {
			return err
		}

		tarReader := tar.NewReader(gzipReader)

		for {
			header, err := tarReader.Next()

			if errors.Is(err, io.EOF) {
				break
			}

			if err != nil {
				return err
			}

			if header.Name != binaryName {
				continue
			}

			binaryBytes, err = io.ReadAll(tarReader)

			if err != nil {
				return err
			}
		}
	}

	if len(binaryBytes) == 0 {
		return errors.New("the release download is missing the binary")
	}

	// Swap the new binary in
	temporary, err := os.CreateTemp(filepath.Dir(executable), ".superstack-upgrade-")

	if err != nil {
		return errors.New("this install cannot be replaced from here, rerun the install script instead")
	}

	_, err = temporary.Write(binaryBytes)

	if err == nil {
		err = temporary.Chmod(0o755)
	}

	if closeError := temporary.Close(); err == nil {
		err = closeError
	}

	if err != nil {
		os.Remove(temporary.Name())
		return err
	}

	// Windows cannot overwrite a running binary but can rename it aside, so
	// the swap goes through a .old that the next upgrade clears
	previous := executable + ".old"

	os.Remove(previous)

	err = os.Rename(executable, previous)

	if err != nil {
		os.Remove(temporary.Name())
		return errors.New("this install cannot be replaced from here, rerun the install script instead")
	}

	err = os.Rename(temporary.Name(), executable)

	if err != nil {
		os.Rename(previous, executable)
		return err
	}

	os.Remove(previous)

	fmt.Printf("Upgraded superstack %s to %s.\n", CliVersion, latestVersion)

	return nil
}
