package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type deviceEntry struct {
	Imei       string  `json:"imei"`
	Name       *string `json:"name"`
	FleetId    int64   `json:"fleet_id"`
	LastSeenAt *string `json:"last_seen_at"`
}

func fetchDevices() ([]deviceEntry, error) {
	request, err := authenticatedRequest(http.MethodGet, "/devices", nil)

	if err != nil {
		return nil, err
	}

	response, err := apiClient.Do(request)

	if err != nil {
		return nil, fmt.Errorf("the server could not be reached: %w", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))

		return nil, fmt.Errorf("the server said: %s", strings.TrimSpace(string(message)))
	}

	devices := []deviceEntry{}

	err = json.NewDecoder(response.Body).Decode(&devices)

	if err != nil {
		return nil, err
	}

	return devices, nil
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
