package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

func DeviceList(arguments []string) error {
	jsonOutput := false
	positionals := []string{}

	for _, argument := range arguments {
		if argument == "--json" {
			jsonOutput = true
			continue
		}

		positionals = append(positionals, argument)
	}

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

	devices, err := fetchDevices()

	if err != nil {
		return err
	}

	fleets, err := fetchFleets()

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

	filtered := []deviceEntry{}

	for _, device := range devices {
		if chosenFleetId == 0 || device.FleetId == chosenFleetId {
			filtered = append(filtered, device)
		}
	}

	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(filtered)
	}

	if len(filtered) == 0 {
		if chosenFleetId == 0 {
			fmt.Println("No devices yet. Claim one with device claim.")
		} else {
			fmt.Println("No devices in that fleet.")
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

			if err != nil {
				return err
			}

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

		imeiValues[index] = device.Imei
		nameValues[index] = name
		fleetValues[index] = fleetNames[device.FleetId]
		stateValues[index] = formatRunState(device.ReportedState)
		storageValues[index] = formatStorage(device.StorageUsed, device.StorageTotal)
		lastSeenValues[index] = lastSeen
		imeiWidth = max(imeiWidth, len(imeiValues[index]))
		nameWidth = max(nameWidth, len(nameValues[index]))
		fleetWidth = max(fleetWidth, len(fleetValues[index]))
		stateWidth = max(stateWidth, len(stateValues[index]))
		storageWidth = max(storageWidth, len(storageValues[index]))
	}

	fmt.Printf("%-*s  %-*s  %-*s  %-*s  %-*s  %s\n",
		imeiWidth, "IMEI", nameWidth, "NAME", fleetWidth, "FLEET",
		stateWidth, "STATE", storageWidth, "STORAGE", "LAST SEEN")

	for index := range filtered {
		fmt.Printf("%-*s  %-*s  %-*s  %-*s  %-*s  %s\n",
			imeiWidth, imeiValues[index], nameWidth, nameValues[index], fleetWidth, fleetValues[index],
			stateWidth, stateValues[index], storageWidth, storageValues[index], lastSeenValues[index])
	}

	return nil
}
