package api

import (
	"encoding/json"
	"errors"
	"net/http"
)

type DeviceEntry struct {
	Imei          string  `json:"imei"`
	Name          *string `json:"name"`
	FleetId       int64   `json:"fleet_id"`
	LastSeenAt    *string `json:"last_seen_at"`
	ReportedState *int    `json:"reported_state"`
	StorageUsed   *int64  `json:"storage_used"`
	StorageTotal  *int64  `json:"storage_total"`
}

func FetchDevices(session Session) ([]DeviceEntry, error) {
	request, err := AuthenticatedRequest(session, http.MethodGet, "/devices", nil)

	if err != nil {
		return nil, err
	}

	response, err := session.Client.Do(request)

	if err != nil {
		return nil, errors.New("the server could not be reached, check your connection")
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, ServerError(response)
	}

	devices := []DeviceEntry{}

	err = json.NewDecoder(response.Body).Decode(&devices)

	if err != nil {
		return nil, err
	}

	return devices, nil
}
