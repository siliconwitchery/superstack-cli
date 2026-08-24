package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

type BalanceEntry struct {
	FleetId  int64  `json:"fleet_id"`
	Balance  string `json:"balance"`
	Currency string `json:"currency"`
}

func FetchBalances(invocation Invocation) ([]BalanceEntry, error) {
	request, err := AuthenticatedRequest(invocation, http.MethodGet, "/balance", nil)

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

	balances := []BalanceEntry{}

	err = Decode(response, &balances)

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
