package device

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/siliconwitchery/superstack-cli/internal/api"
)

func Claim(session api.Session, arguments []string) error {
	if len(arguments) != 2 && len(arguments) != 3 {
		return errors.New("device claim takes an IMEI, a fleet id, and an optional name")
	}

	imei := arguments[0]

	if !validImei(imei) {
		return errors.New("the IMEI is the 15-digit number printed on the device")
	}

	fleetId, err := strconv.ParseInt(arguments[1], 10, 64)

	if err != nil || fleetId < 1 {
		return errors.New("the fleet id is the number shown by fleet list")
	}

	fleets, err := api.FetchFleets(session)

	if err != nil {
		return err
	}

	fleetName := ""

	for _, fleet := range fleets {
		if fleet.Id == fleetId {
			fleetName = fleet.Name
		}
	}

	if fleetName == "" {
		return errors.New("no such fleet")
	}

	fmt.Fprintln(session.Out, "Press the pairing button on the device to finish claiming it.")

	payload := map[string]string{"imei": imei}

	if len(arguments) == 3 {
		payload["name"] = arguments[2]
	}

	body, err := json.Marshal(payload)

	if err != nil {
		return err
	}

	request, err := api.AuthenticatedRequest(session, http.MethodPost,
		"/fleets/"+strconv.FormatInt(fleetId, 10)+"/devices", bytes.NewReader(body))

	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/json")

	claimClient := &http.Client{Timeout: 90 * time.Second}

	response, err := claimClient.Do(request)

	if err != nil {
		return errors.New("the server could not be reached, check your connection")
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusNoContent {
		return api.ServerError(response)
	}

	fmt.Fprintf(session.Out, "Claimed device %s into fleet %q.\n", imei, fleetName)

	return nil
}

func List(session api.Session, arguments []string) error {
	positionals, jsonOutput := api.TakeJsonFlag(arguments)

	if len(positionals) > 1 {
		return errors.New("device list takes at most one fleet id")
	}

	chosenFleetId := int64(0)

	if len(positionals) == 1 {
		parsed, err := strconv.ParseInt(positionals[0], 10, 64)

		if err != nil || parsed < 1 {
			return errors.New("the fleet id is the number shown by fleet list")
		}

		chosenFleetId = parsed
	}

	devices, err := api.FetchDevices(session)

	if err != nil {
		return err
	}

	fleets, err := api.FetchFleets(session)

	if err != nil {
		return err
	}

	fleetNames := map[int64]string{}

	for _, fleet := range fleets {
		fleetNames[fleet.Id] = fleet.Name
	}

	if chosenFleetId != 0 {
		if _, found := fleetNames[chosenFleetId]; !found {
			return errors.New("no such fleet")
		}
	}

	filtered := []api.DeviceEntry{}

	for _, device := range devices {
		if chosenFleetId == 0 || device.FleetId == chosenFleetId {
			filtered = append(filtered, device)
		}
	}

	if jsonOutput {
		err = json.NewEncoder(session.Out).Encode(filtered)

		return err
	}

	if len(filtered) == 0 {
		if chosenFleetId == 0 {
			fmt.Fprintln(session.Out, "No devices yet. Claim one with device claim.")
		} else {
			fmt.Fprintln(session.Out, "No devices in that fleet.")
		}

		return nil
	}

	imeiWidth := len("IMEI")
	nameWidth := len("NAME")
	fleetWidth := len("FLEET")
	stateWidth := len("STATE")
	storageWidth := len("STORAGE")
	imeiValues := make([]string, len(filtered))
	nameValues := make([]string, len(filtered))
	fleetValues := make([]string, len(filtered))
	stateValues := make([]string, len(filtered))
	storageValues := make([]string, len(filtered))
	lastSeenValues := make([]string, len(filtered))

	for index, device := range filtered {
		name := "-"

		if device.Name != nil {
			name = *device.Name
		}

		lastSeen := "never"

		if device.LastSeenAt != nil {
			seenAt, err := time.Parse(time.RFC3339, *device.LastSeenAt)

			lastSeen = "unknown"

			if err == nil {
				age := time.Since(seenAt)

				switch {
				case age < 2*time.Minute:
					lastSeen = "just now"
				case age < time.Hour:
					lastSeen = fmt.Sprintf("%d min ago", int(age.Minutes()))
				case age < 24*time.Hour:
					lastSeen = fmt.Sprintf("%d h ago", int(age.Hours()))
				default:
					lastSeen = fmt.Sprintf("%d d ago", int(age.Hours()/24))
				}
			}
		}

		state := "unknown"

		if device.ReportedState != nil {
			switch *device.ReportedState {
			case 2:
				state = "running"
			case 3:
				state = "stopped"
			case 4:
				state = "crashed"
			}
		}

		storage := "-"

		if device.StorageUsed != nil && device.StorageTotal != nil {
			formatBytes := func(bytes int64) string {
				switch {
				case bytes < 1000:
					return fmt.Sprintf("%d B", bytes)
				case bytes < 1000*1000:
					return fmt.Sprintf("%.1f kB", float64(bytes)/1000)
				default:
					return fmt.Sprintf("%.1f MB", float64(bytes)/(1000*1000))
				}
			}

			storage = fmt.Sprintf("%s of %s", formatBytes(*device.StorageUsed), formatBytes(*device.StorageTotal))
		}

		fleetName, known := fleetNames[device.FleetId]

		if !known {
			fleetName = "-"
		}

		imeiValues[index] = api.Printable(device.Imei)
		nameValues[index] = api.Printable(name)
		fleetValues[index] = api.Printable(fleetName)
		stateValues[index] = state
		storageValues[index] = storage
		lastSeenValues[index] = lastSeen
		imeiWidth = max(imeiWidth, len(imeiValues[index]))
		nameWidth = max(nameWidth, len(nameValues[index]))
		fleetWidth = max(fleetWidth, len(fleetValues[index]))
		stateWidth = max(stateWidth, len(stateValues[index]))
		storageWidth = max(storageWidth, len(storageValues[index]))
	}

	fmt.Fprintf(session.Out, "%-*s  %-*s  %-*s  %-*s  %-*s  %s\n",
		imeiWidth, "IMEI", nameWidth, "NAME", fleetWidth, "FLEET",
		stateWidth, "STATE", storageWidth, "STORAGE", "LAST SEEN")

	for index := range filtered {
		fmt.Fprintf(session.Out, "%-*s  %-*s  %-*s  %-*s  %-*s  %s\n",
			imeiWidth, imeiValues[index], nameWidth, nameValues[index], fleetWidth, fleetValues[index],
			stateWidth, stateValues[index], storageWidth, storageValues[index], lastSeenValues[index])
	}

	return nil
}

func Rename(session api.Session, arguments []string) error {
	if len(arguments) != 2 {
		return errors.New("device rename takes an IMEI and a new name, quoted if it has spaces")
	}

	imei := arguments[0]

	if !validImei(imei) {
		return errors.New("the IMEI is the 15-digit number printed on the device")
	}

	name := strings.TrimSpace(arguments[1])

	if name == "" {
		return errors.New("device rename takes an IMEI and a new name, quoted if it has spaces")
	}

	body, err := json.Marshal(map[string]string{"name": name})

	if err != nil {
		return err
	}

	request, err := api.AuthenticatedRequest(session, http.MethodPatch, "/devices/"+imei, bytes.NewReader(body))

	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := session.Client.Do(request)

	if err != nil {
		return errors.New("the server could not be reached, check your connection")
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusNoContent {
		return api.ServerError(response)
	}

	fmt.Fprintf(session.Out, "Renamed device %s to %q.\n", imei, name)

	return nil
}

func Release(session api.Session, arguments []string) error {
	if len(arguments) != 1 {
		return errors.New("device release takes an IMEI")
	}

	imei := arguments[0]

	if !validImei(imei) {
		return errors.New("the IMEI is the 15-digit number printed on the device")
	}

	devices, err := api.FetchDevices(session)

	if err != nil {
		return err
	}

	fleetId := int64(0)
	label := ""

	for _, device := range devices {
		if device.Imei != imei {
			continue
		}

		fleetId = device.FleetId
		label = imei

		if device.Name != nil && *device.Name != "" {
			label = *device.Name
		}
	}

	if fleetId == 0 {
		return errors.New("no such device, device list shows yours")
	}

	fleets, err := api.FetchFleets(session)

	if err != nil {
		return err
	}

	fleetName := ""

	for _, fleet := range fleets {
		if fleet.Id == fleetId {
			fleetName = fleet.Name
		}
	}

	if fleetName == "" {
		return errors.New("no such device, device list shows yours")
	}

	fmt.Fprintf(session.Out, "Release device %q from fleet %q? It wipes the device's files and restarts its code, and claiming it again means pressing its pairing button in person. [y/N] ", label, fleetName)

	answer, _ := bufio.NewReader(session.In).ReadString('\n')

	answer = strings.ToLower(strings.TrimSpace(answer))

	if answer != "y" && answer != "yes" {
		fmt.Fprintln(session.Out, "Nothing released.")
		return nil
	}

	request, err := api.AuthenticatedRequest(session, http.MethodDelete, "/devices/"+imei, nil)

	if err != nil {
		return err
	}

	response, err := session.Client.Do(request)

	if err != nil {
		return errors.New("the server could not be reached, check your connection")
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusNoContent {
		return api.ServerError(response)
	}

	fmt.Fprintf(session.Out, "Released device %q from fleet %q.\n", label, fleetName)

	return nil
}

func validImei(imei string) bool {
	if len(imei) != 15 {
		return false
	}

	for _, digit := range imei {
		if digit < '0' || digit > '9' {
			return false
		}
	}

	return true
}
