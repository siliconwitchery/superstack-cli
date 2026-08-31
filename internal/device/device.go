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

func Pair(invocation api.Invocation, arguments []string) error {
	if len(arguments) < 2 || len(arguments) > 3 {
		return errors.New("device pair takes an IMEI, a fleet id, and an optional name")
	}

	imei := arguments[0]

	if !validImei(imei) {
		return errors.New("the IMEI is the 15-digit number printed on the device")
	}

	fleetID, err := strconv.ParseInt(arguments[1], 10, 64)

	if err != nil || fleetID < 1 {
		return errors.New("the fleet id is the number shown by fleet list")
	}

	requestBody := struct {
		IMEI string  `json:"imei"`
		Name *string `json:"name,omitempty"`
	}{
		IMEI: imei,
	}

	label := imei

	if len(arguments) == 3 {
		name := strings.TrimSpace(arguments[2])

		if name == "" {
			return errors.New("the optional device name cannot be empty")
		}

		requestBody.Name = &name
		label = name
	}

	body, err := json.Marshal(requestBody)

	if err != nil {
		return err
	}

	request, err := api.AuthenticatedRequest(invocation, http.MethodPost,
		"/fleets/"+strconv.FormatInt(fleetID, 10)+"/devices", bytes.NewReader(body))

	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/json")

	fmt.Fprintf(invocation.Out, "Press the pairing button on device %q.\n", label)

	client := *invocation.Client

	if client.Timeout > 0 && client.Timeout < 65*time.Second {
		client.Timeout = 65 * time.Second
	}

	response, err := client.Do(request)

	if err != nil {
		return errors.New("the server could not be reached, check your internet access")
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusNoContent {
		return api.ServerError(response)
	}

	fmt.Fprintf(invocation.Out, "Paired device %q with fleet %d.\n", label, fleetID)

	return nil
}

func List(invocation api.Invocation, arguments []string) error {
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

	devices, err := api.FetchDevices(invocation)

	if err != nil {
		return err
	}

	fleets, err := api.FetchFleets(invocation)

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
		err = json.NewEncoder(invocation.Out).Encode(filtered)

		return err
	}

	if len(filtered) == 0 {
		if chosenFleetId == 0 {
			fmt.Fprintln(invocation.Out, "No devices yet.")
		} else {
			fmt.Fprintln(invocation.Out, "No devices in that fleet.")
		}

		return nil
	}

	imeiWidth := len("IMEI")
	nameWidth := len("NAME")
	fleetWidth := len("FLEET")
	runStateWidth := len("RUN STATE")
	storageWidth := len("STORAGE")
	imeiValues := make([]string, len(filtered))
	nameValues := make([]string, len(filtered))
	fleetValues := make([]string, len(filtered))
	runStateValues := make([]string, len(filtered))
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

		fleetName, known := fleetNames[device.FleetId]

		if !known {
			fleetName = "-"
		}

		runState := "-"

		if device.RunState != nil {
			runState = *device.RunState
		}

		storage := "-"

		if device.StorageUsed != nil && device.StorageTotal != nil {
			storage = fmt.Sprintf("%s / %s", formatByteCount(*device.StorageUsed), formatByteCount(*device.StorageTotal))
		}

		imeiValues[index] = api.Printable(device.Imei)
		nameValues[index] = api.Printable(name)
		fleetValues[index] = api.Printable(fleetName)
		runStateValues[index] = runState
		storageValues[index] = storage
		lastSeenValues[index] = lastSeen
		imeiWidth = max(imeiWidth, len(imeiValues[index]))
		nameWidth = max(nameWidth, len(nameValues[index]))
		fleetWidth = max(fleetWidth, len(fleetValues[index]))
		runStateWidth = max(runStateWidth, len(runStateValues[index]))
		storageWidth = max(storageWidth, len(storageValues[index]))
	}

	fmt.Fprintf(invocation.Out, "%-*s  %-*s  %-*s  %-*s  %-*s  %s\n",
		imeiWidth, "IMEI", nameWidth, "NAME", fleetWidth, "FLEET", runStateWidth, "RUN STATE",
		storageWidth, "STORAGE", "LAST SEEN")

	for index := range filtered {
		fmt.Fprintf(invocation.Out, "%-*s  %-*s  %-*s  %-*s  %-*s  %s\n",
			imeiWidth, imeiValues[index], nameWidth, nameValues[index], fleetWidth, fleetValues[index],
			runStateWidth, runStateValues[index], storageWidth, storageValues[index], lastSeenValues[index])
	}

	return nil
}

func formatByteCount(byteCount uint64) string {
	units := [...]string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	value := float64(byteCount)
	unit := 0

	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}

	if unit == 0 {
		return fmt.Sprintf("%d B", byteCount)
	}

	if value >= 10 {
		return fmt.Sprintf("%.0f %s", value, units[unit])
	}

	return fmt.Sprintf("%.1f %s", value, units[unit])
}

func Rename(invocation api.Invocation, arguments []string) error {
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

	request, err := api.AuthenticatedRequest(invocation, http.MethodPatch, "/devices/"+imei, bytes.NewReader(body))

	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := invocation.Client.Do(request)

	if err != nil {
		return errors.New("the server could not be reached, check your internet access")
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusNoContent {
		return api.ServerError(response)
	}

	fmt.Fprintf(invocation.Out, "Renamed device %s to %q.\n", imei, name)

	return nil
}

func Unpair(invocation api.Invocation, arguments []string) error {
	if len(arguments) != 1 {
		return errors.New("device unpair takes an IMEI")
	}

	imei := arguments[0]

	if !validImei(imei) {
		return errors.New("the IMEI is the 15-digit number printed on the device")
	}

	devices, err := api.FetchDevices(invocation)

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

	fleets, err := api.FetchFleets(invocation)

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

	fmt.Fprintf(invocation.Out, "Unpair device %q from fleet %q? It will no longer appear in the fleet. [y/N] ", label, fleetName)

	answer, _ := bufio.NewReader(invocation.In).ReadString('\n')

	answer = strings.ToLower(strings.TrimSpace(answer))

	if answer != "y" && answer != "yes" {
		fmt.Fprintln(invocation.Out, "Nothing unpaired.")
		return nil
	}

	request, err := api.AuthenticatedRequest(invocation, http.MethodDelete, "/devices/"+imei, nil)

	if err != nil {
		return err
	}

	response, err := invocation.Client.Do(request)

	if err != nil {
		return errors.New("the server could not be reached, check your internet access")
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusNoContent {
		return api.ServerError(response)
	}

	fmt.Fprintf(invocation.Out, "Unpaired device %q from fleet %q.\n", label, fleetName)

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
