package commands

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestFetchBalancesFailures(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		wantError string
	}{
		{name: "server refusal", status: http.StatusServiceUnavailable, body: "balances unavailable", wantError: "balances unavailable"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("GET /balance", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.status)
				fmt.Fprint(w, test.body)
			})

			session, _ := loggedInSession(t, mux)

			_, err := fetchBalances(session)

			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("error = %v, want it to mention %q", err, test.wantError)
			}
		})
	}
}

func TestFormatBalance(t *testing.T) {
	tests := []struct {
		balance   string
		want      string
		wantValue float64
		wantValid bool
	}{
		{balance: "15.000000", want: "\u20ac15.00", wantValue: 15, wantValid: true},
		{balance: "0", want: "\u20ac0.00", wantValue: 0, wantValid: true},
		{balance: "0.004", want: "\u20ac0.00", wantValue: 0.004, wantValid: true},
		{balance: "-2.50", want: "\u20ac-2.50", wantValue: -2.5, wantValid: true},
		{balance: "", want: "", wantValue: 0, wantValid: false},
		{balance: "not a number", want: "not a number", wantValue: 0, wantValid: false},
	}

	for _, test := range tests {
		t.Run(test.balance, func(t *testing.T) {
			formatted, value, valid := formatBalance(balanceEntry{Balance: test.balance})

			if formatted != test.want || value != test.wantValue || valid != test.wantValid {
				t.Errorf("formatBalance() = %q, %v, %v, want %q, %v, %v",
					formatted, value, valid, test.want, test.wantValue, test.wantValid)
			}
		})
	}
}
