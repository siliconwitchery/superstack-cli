package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

type balanceEntry struct {
	Fleet    int64  `json:"fleet"`
	Balance  string `json:"balance"`
	Currency string `json:"currency"`
}

func fetchBalances(session Session) ([]balanceEntry, error) {
	request, err := authenticatedRequest(session, http.MethodGet, "/balance", nil)

	if err != nil {
		return nil, err
	}

	response, err := session.Client.Do(request)

	if err != nil {
		return nil, fmt.Errorf("the server could not be reached: %w", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, serverError(response)
	}

	balances := []balanceEntry{}

	err = json.NewDecoder(response.Body).Decode(&balances)

	if err != nil {
		return nil, err
	}

	return balances, nil
}

func formatBalance(entry balanceEntry) string {
	value, err := strconv.ParseFloat(entry.Balance, 64)

	if err != nil {
		return entry.Balance
	}

	return fmt.Sprintf("€%.2f", value)
}
