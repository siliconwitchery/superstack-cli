package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type deviceEntry struct {
	Imei          string  `json:"imei"`
	Name          *string `json:"name"`
	FleetId       int64   `json:"fleet_id"`
	LastSeenAt    *string `json:"last_seen_at"`
	ReportedState *int    `json:"reported_state"`
	StorageUsed   *int64  `json:"storage_used"`
	StorageTotal  *int64  `json:"storage_total"`
}

func fetchDevices(session Session) ([]deviceEntry, error) {
	request, err := authenticatedRequest(session, http.MethodGet, "/devices", nil)

	if err != nil {
		return nil, err
	}

	response, err := session.Client.Do(request)

	if err != nil {
		return nil, errors.New("the server could not be reached, check your connection")
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, serverError(response)
	}

	devices := []deviceEntry{}

	err = json.NewDecoder(response.Body).Decode(&devices)

	if err != nil {
		return nil, err
	}

	return devices, nil
}

func formatRunState(state *int) string {
	if state == nil {
		return "unknown"
	}

	switch *state {
	case 2:
		return "running"
	case 3:
		return "stopped"
	case 4:
		return "crashed"
	default:
		return "unknown"
	}
}

func formatStorage(used *int64, total *int64) string {
	if used == nil || total == nil {
		return "-"
	}

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

	return fmt.Sprintf("%s of %s", formatBytes(*used), formatBytes(*total))
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
