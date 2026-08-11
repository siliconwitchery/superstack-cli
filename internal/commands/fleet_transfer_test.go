package commands

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestFleetTransfer(t *testing.T) {
	transferredPath := ""
	transferredTo := ""

	mux := http.NewServeMux()

	mux.HandleFunc("POST /fleets/{id}/owner", func(w http.ResponseWriter, r *http.Request) {
		body := struct {
			Email string `json:"email"`
		}{}

		json.NewDecoder(r.Body).Decode(&body)

		transferredPath = r.URL.Path
		transferredTo = body.Email

		w.WriteHeader(http.StatusNoContent)
	})

	loggedInTestServer(t, mux)

	err := FleetTransfer([]string{"3", "successor@example.com"})

	if err != nil {
		t.Fatal(err)
	}

	if transferredPath != "/fleets/3/owner" || transferredTo != "successor@example.com" {
		t.Errorf("the server saw %q handed to %q, want %q handed to %q",
			transferredPath, transferredTo, "/fleets/3/owner", "successor@example.com")
	}
}

func TestFleetTransferArguments(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
		wantError string
	}{
		{"no arguments", nil, "takes a fleet id and an email address"},
		{"only an id", []string{"3"}, "takes a fleet id and an email address"},
		{"an empty address", []string{"3", ""}, "takes a fleet id and an email address"},
		{"a wordy id", []string{"pilot", "successor@example.com"}, "shown by fleet list"},
	}

	for _, test := range tests {
		err := FleetTransfer(test.arguments)

		if err == nil || !strings.Contains(err.Error(), test.wantError) {
			t.Errorf("%s: error = %v, want it to mention %q", test.name, err, test.wantError)
		}
	}
}
