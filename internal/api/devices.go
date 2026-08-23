package api

import (
	"errors"
	"net/http"
)

type DeviceEntry struct {
	Imei         string  `json:"imei"`
	Name         *string `json:"name"`
	FleetId      int64   `json:"fleet_id"`
	LastSeenAt   *string `json:"last_seen_at"`
	RunState     *int    `json:"run_state"`
	StorageUsed  *int64  `json:"storage_used"`
	StorageTotal *int64  `json:"storage_total"`
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

	serverDevices := []struct {
		Imei         string  `json:"imei"`
		Name         *string `json:"name"`
		FleetId      int64   `json:"fleet_id"`
		LastSeenAt   *string `json:"last_seen_at"`
		RunState     *int    `json:"reported_state"`
		StorageUsed  *int64  `json:"storage_used"`
		StorageTotal *int64  `json:"storage_total"`
	}{}

	err = Decode(response, &serverDevices)

	if err != nil {
		return nil, err
	}

	devices := make([]DeviceEntry, len(serverDevices))

	for index, device := range serverDevices {
		devices[index] = DeviceEntry{
			Imei:         device.Imei,
			Name:         device.Name,
			FleetId:      device.FleetId,
			LastSeenAt:   device.LastSeenAt,
			RunState:     device.RunState,
			StorageUsed:  device.StorageUsed,
			StorageTotal: device.StorageTotal,
		}
	}

	return devices, nil
}
