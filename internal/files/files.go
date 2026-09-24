package files

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/siliconwitchery/superstack-cli/internal/api"
)

func Upload(invocation api.Invocation, arguments []string) error {
	if len(arguments) < 2 {
		return errors.New("upload takes an IMEI or fleet id, then the files or directories to upload")
	}

	target := arguments[0]
	isImei := len(target) == 15 && !strings.ContainsFunc(target, func(digit rune) bool { return digit < '0' || digit > '9' })
	fleetID := int64(0)

	if !isImei {
		parsed, err := strconv.ParseInt(target, 10, 64)

		if err != nil || parsed < 1 {
			return errors.New("the first argument is the 15-digit IMEI printed on the device, or the fleet id shown by fleet list")
		}

		fleetID = parsed
	}

	collected := map[string][]byte{}
	sources := map[string]string{}

	add := func(devicePath string, localPath string) error {
		if previous, seen := sources[devicePath]; seen {
			return fmt.Errorf("%s would be uploaded twice, from %s and %s", devicePath, previous, localPath)
		}

		content, err := os.ReadFile(localPath)

		if err != nil {
			return fmt.Errorf("%s could not be read", localPath)
		}

		collected[devicePath] = content
		sources[devicePath] = localPath

		return nil
	}

	for _, argument := range arguments[1:] {
		info, err := os.Stat(argument)

		if err != nil {
			return fmt.Errorf("%s does not exist", argument)
		}

		if !info.IsDir() {
			err = add(filepath.Base(argument), argument)

			if err != nil {
				return err
			}

			continue
		}

		err = filepath.WalkDir(argument, func(path string, entry fs.DirEntry, walkError error) error {
			if walkError != nil {
				return fmt.Errorf("%s could not be read", path)
			}

			if path == argument {
				return nil
			}

			if strings.HasPrefix(entry.Name(), ".") {
				if entry.IsDir() {
					return filepath.SkipDir
				}

				return nil
			}

			if !entry.Type().IsRegular() {
				return nil
			}

			relative, err := filepath.Rel(argument, path)

			if err != nil {
				return fmt.Errorf("%s could not be read", path)
			}

			return add(filepath.ToSlash(relative), path)
		})

		if err != nil {
			return err
		}
	}

	if len(collected) == 0 {
		return errors.New("there are no files to upload")
	}

	body, err := json.Marshal(struct {
		Files map[string][]byte `json:"files"`
	}{collected})

	if err != nil {
		return err
	}

	path := "/devices/" + target + "/files"

	if !isImei {
		path = "/fleets/" + strconv.FormatInt(fleetID, 10) + "/files"
	}

	request, err := api.AuthenticatedRequest(invocation, http.MethodPut, path, bytes.NewReader(body))

	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := invocation.Client.Do(request)

	if err != nil {
		return errors.New("the server could not be reached, check your internet access")
	}

	defer response.Body.Close()

	fileNoun := "files"
	arrival := "They arrive"

	if len(collected) == 1 {
		fileNoun = "file"
		arrival = "It arrives"
	}

	if isImei {
		if response.StatusCode != http.StatusNoContent {
			return api.ServerError(response)
		}

		fmt.Fprintf(invocation.Out, "Uploaded %d %s to device %s. %s at its next check-in.\n",
			len(collected), fileNoun, target, arrival)

		return nil
	}

	if response.StatusCode != http.StatusOK {
		return api.ServerError(response)
	}

	result := struct {
		Devices int `json:"devices"`
	}{}

	err = api.Decode(response, &result)

	if err != nil {
		return err
	}

	deviceNoun := "devices"

	if result.Devices == 1 {
		deviceNoun = "device"
	}

	fmt.Fprintf(invocation.Out, "Uploaded %d %s to %d %s in fleet %d. %s at each device's next check-in.\n",
		len(collected), fileNoun, result.Devices, deviceNoun, fleetID, arrival)

	return nil
}

func Download(invocation api.Invocation, arguments []string) error {
	if len(arguments) != 2 {
		return errors.New("download takes an IMEI, then the directory to download into")
	}

	imei := arguments[0]
	isImei := len(imei) == 15 && !strings.ContainsFunc(imei, func(digit rune) bool { return digit < '0' || digit > '9' })

	if !isImei {
		return errors.New("download names one device by the 15-digit IMEI printed on it, not a fleet")
	}

	destination := arguments[1]
	info, err := os.Stat(destination)

	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s could not be read", destination)
	}

	if err == nil && !info.IsDir() {
		return fmt.Errorf("%s is a file, choose a directory", destination)
	}

	request, err := api.AuthenticatedRequest(invocation, http.MethodGet, "/devices/"+imei+"/files", nil)

	if err != nil {
		return err
	}

	response, err := invocation.Client.Do(request)

	if err != nil {
		return errors.New("the server could not be reached, check your internet access")
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return api.ServerError(response)
	}

	result := struct {
		Files map[string][]byte `json:"files"`
	}{}

	err = api.Decode(response, &result)

	if err != nil {
		return err
	}

	paths := make([]string, 0, len(result.Files))

	for path := range result.Files {
		if path == "" || strings.HasPrefix(path, "/") || filepath.IsAbs(filepath.FromSlash(path)) || slices.Contains(strings.Split(path, "/"), "..") {
			return errors.New("the download could not be trusted, so nothing was written")
		}

		paths = append(paths, path)
	}

	slices.Sort(paths)

	root := filepath.Clean(destination)

	for _, path := range paths {
		localPath := filepath.Join(root, filepath.FromSlash(path))

		for parent := filepath.Dir(localPath); parent != root; parent = filepath.Dir(parent) {
			info, err := os.Stat(parent)

			if err == nil && !info.IsDir() {
				return fmt.Errorf("%s is a file where the code needs a directory", parent)
			}
		}
	}

	existing := []string{}

	for _, path := range paths {
		localPath := filepath.Join(destination, filepath.FromSlash(path))
		_, err := os.Stat(localPath)

		if err == nil {
			existing = append(existing, localPath)
		}
	}

	if len(existing) == 1 {
		fmt.Fprintf(invocation.Out, "%s already exists. Replace it? [y/N] ", existing[0])
	} else if len(existing) > 1 {
		fmt.Fprintf(invocation.Out, "These files already exist:\n%s\nReplace them? [y/N] ", strings.Join(existing, "\n"))
	}

	if len(existing) > 0 {
		answer, _ := bufio.NewReader(invocation.In).ReadString('\n')

		answer = strings.ToLower(strings.TrimSpace(answer))

		if answer != "y" && answer != "yes" {
			fmt.Fprintln(invocation.Out, "Nothing downloaded.")
			return nil
		}
	}

	for _, path := range paths {
		localPath := filepath.Join(destination, filepath.FromSlash(path))
		err = os.MkdirAll(filepath.Dir(localPath), 0o755)

		if err != nil {
			return fmt.Errorf("%s could not be created", filepath.Dir(localPath))
		}

		err = os.WriteFile(localPath, result.Files[path], 0o644)

		if err != nil {
			return fmt.Errorf("%s could not be written", localPath)
		}

		fmt.Fprintln(invocation.Out, localPath)
	}

	fileNoun := "files"

	if len(paths) == 1 {
		fileNoun = "file"
	}

	fmt.Fprintf(invocation.Out, "Downloaded %d %s from device %s into %s.\n", len(paths), fileNoun, imei, destination)

	return nil
}
