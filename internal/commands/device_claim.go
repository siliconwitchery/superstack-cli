package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

func DeviceClaim(session Session, arguments []string) error {
	claimClient := &http.Client{Timeout: 90 * time.Second}

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

	fleets, err := fetchFleets(session)

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
		return errors.New("the fleet id is the number shown by fleet list")
	}

	fmt.Fprintln(session.Out, "Press the button on the device to finish claiming it.")

	payload := map[string]string{"imei": imei}

	if len(arguments) == 3 {
		payload["name"] = arguments[2]
	}

	body, err := json.Marshal(payload)

	if err != nil {
		return err
	}

	request, err := authenticatedRequest(session, http.MethodPost,
		"/fleets/"+strconv.FormatInt(fleetId, 10)+"/devices", bytes.NewReader(body))

	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := claimClient.Do(request)

	if err != nil {
		return fmt.Errorf("the server could not be reached: %w", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusNoContent {
		return serverError(response)
	}

	fmt.Fprintf(session.Out, "Claimed the device into %q.\n", fleetName)

	return nil
}
