package api

import (
	"errors"
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

// position is last, for the newest logs, or after, for the logs newer than a cursor
func FetchLogs(invocation Invocation, fleetID int64, imeis []string, position string, value int64) (LogsAnswer, error) {
	query := url.Values{position: {strconv.FormatInt(value, 10)}}

	for _, imei := range imeis {
		query.Add("imei", imei)
	}

	request, err := AuthenticatedRequest(invocation, http.MethodGet,
		"/fleets/"+strconv.FormatInt(fleetID, 10)+"/logs?"+query.Encode(), nil)

	if err != nil {
		return LogsAnswer{}, err
	}

	response, err := invocation.Client.Do(request)

	if err != nil {
		return LogsAnswer{}, errors.New("the server could not be reached, check your internet access")
	}

	defer response.Body.Close()

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
