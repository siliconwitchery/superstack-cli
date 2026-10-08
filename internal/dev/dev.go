package dev

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/siliconwitchery/superstack-cli/internal/api"
	"github.com/siliconwitchery/superstack-cli/internal/files"
	"github.com/siliconwitchery/superstack-cli/internal/tail"
)

type serialWriter struct {
	mutex sync.Mutex
	out   io.Writer
}

func (writer *serialWriter) Write(data []byte) (int, error) {
	writer.mutex.Lock()
	defer writer.mutex.Unlock()

	return writer.out.Write(data)
}

func Dev(invocation api.Invocation, arguments []string) error {
	if len(arguments) < 2 {
		return errors.New("dev takes an IMEI or fleet id, then the files or directories to upload on every change")
	}

	target := arguments[0]
	tailArguments := []string{target}
	isImei := len(target) == 15 && !strings.ContainsFunc(target, func(digit rune) bool { return digit < '0' || digit > '9' })

	if isImei {
		devices, err := api.FetchDevices(invocation)

		if err != nil {
			return err
		}

		index := slices.IndexFunc(devices, func(device api.DeviceEntry) bool { return device.Imei == target })

		if index < 0 {
			return fmt.Errorf("device %s is not paired with any of your fleets", target)
		}

		tailArguments = []string{strconv.FormatInt(devices[index].FleetId, 10), target}
	}

	invocation.Out = &serialWriter{out: invocation.Out}

	// The first upload, which also checks the arguments
	err := files.Upload(invocation, arguments)

	if err != nil {
		return err
	}

	// The log, whose end is the end of dev
	tailEnded := make(chan error, 1)

	go func() {
		tailEnded <- tail.Tail(invocation, tailArguments)
	}()

	// Upload again once the watched paths have changed and then stayed still for one poll
	previous := snapshot(arguments[1:])
	changed := false

	for {
		select {
		case err := <-tailEnded:
			return err

		case <-time.After(500 * time.Millisecond):
		}

		current := snapshot(arguments[1:])

		if !maps.Equal(current, previous) {
			previous = current
			changed = true
			continue
		}

		if !changed {
			continue
		}

		changed = false
		err = files.Upload(invocation, arguments)

		if err != nil {
			fmt.Fprintf(invocation.Out, "superstack: %s\n", err)
		}
	}
}

func snapshot(paths []string) map[string]string {
	state := map[string]string{}

	for _, argument := range paths {
		filepath.WalkDir(argument, func(path string, entry fs.DirEntry, walkError error) error {
			if walkError != nil {
				return nil
			}

			if path != argument && strings.HasPrefix(entry.Name(), ".") {
				if entry.IsDir() {
					return filepath.SkipDir
				}

				return nil
			}

			if !entry.Type().IsRegular() {
				return nil
			}

			info, err := entry.Info()

			if err != nil {
				return nil
			}

			state[path] = strconv.FormatInt(info.Size(), 10) + " " + info.ModTime().String()

			return nil
		})
	}

	return state
}
