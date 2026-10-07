package api

import (
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type LogEntry struct {
	Id         int64     `json:"id"`
	Imei       string    `json:"imei"`
	Name       *string   `json:"name"`
	Kind       string    `json:"kind"`
	Text       string    `json:"text"`
	ReceivedAt time.Time `json:"received_at"`
}

type LogsAnswer struct {
	Logs []LogEntry `json:"logs"`
	Next int64      `json:"next"`
}

// A request that failed because the server could not be reached or is not available, so a follower may try again
type Unavailable struct {
	Reason string
}

func (unavailable Unavailable) Error() string {
	return unavailable.Reason
}

// query holds last and optionally offset, or after, and any imei values that narrow the fleet
func FetchLogs(invocation Invocation, fleetID int64, query url.Values) (LogsAnswer, error) {
	request, err := AuthenticatedRequest(invocation, http.MethodGet,
		"/fleets/"+strconv.FormatInt(fleetID, 10)+"/logs?"+query.Encode(), nil)

	if err != nil {
		return LogsAnswer{}, err
	}

	response, err := invocation.Client.Do(request)

	if err != nil {
		return LogsAnswer{}, Unavailable{"the server could not be reached, check your internet access"}
	}

	defer response.Body.Close()

	if response.StatusCode == http.StatusBadGateway || response.StatusCode == http.StatusServiceUnavailable ||
		response.StatusCode == http.StatusGatewayTimeout {
		return LogsAnswer{}, Unavailable{ServerError(response).Error()}
	}

	if response.StatusCode != http.StatusOK {
		return LogsAnswer{}, ServerError(response)
	}

	answer := LogsAnswer{}

	err = Decode(response, &answer)

	if err != nil {
		return LogsAnswer{}, err
	}

	return answer, nil
}
