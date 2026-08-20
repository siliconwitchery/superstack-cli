package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

type BalanceEntry struct {
	Fleet    int64  `json:"fleet"`
	Balance  string `json:"balance"`
	Currency string `json:"currency"`
}

func FetchBalances(session Session) ([]BalanceEntry, error) {
	request, err := AuthenticatedRequest(session, http.MethodGet, "/balance", nil)

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

	balances := []BalanceEntry{}

	err = json.NewDecoder(response.Body).Decode(&balances)

	if err != nil {
		return nil, err
	}

	return balances, nil
}

func FormatBalance(entry BalanceEntry) (string, float64, bool) {
	value, err := strconv.ParseFloat(entry.Balance, 64)

	if err != nil {
		return entry.Balance, 0, false
	}

	return fmt.Sprintf("€%.2f", value), value, true
}
