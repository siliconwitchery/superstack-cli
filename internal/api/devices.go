package api

import (
	"errors"
	"net/http"
)

type DeviceEntry struct {
	Imei       string  `json:"imei"`
	Name       *string `json:"name"`
	FleetId    int64   `json:"fleet_id"`
	LastSeenAt *string `json:"last_seen_at"`
}

func FetchDevices(invocation Invocation) ([]DeviceEntry, error) {
	request, err := AuthenticatedRequest(invocation, http.MethodGet, "/devices", nil)

	if err != nil {
		return nil, err
	}

	response, err := invocation.Client.Do(request)

	if err != nil {
		return nil, errors.New("the server could not be reached, check your internet access")
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, ServerError(response)
	}

	devices := []DeviceEntry{}

	err = Decode(response, &devices)

	if err != nil {
		return nil, err
	}

	return devices, nil
}
