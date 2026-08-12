package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type balanceEntry struct {
	Fleet    int64  `json:"fleet"`
	Balance  string `json:"balance"`
	Currency string `json:"currency"`
}

func fetchBalances() ([]balanceEntry, error) {
	request, err := authenticatedRequest(http.MethodGet, "/balance", nil)

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
