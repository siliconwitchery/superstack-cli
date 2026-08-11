package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestFleetCreate(t *testing.T) {
	created := ""

	mux := http.NewServeMux()

	mux.HandleFunc("POST /fleets", func(w http.ResponseWriter, r *http.Request) {
		body := struct {
			Name string `json:"name"`
		}{}

		json.NewDecoder(r.Body).Decode(&body)

		created = body.Name

		fmt.Fprintf(w, `{"id": 5, "name": %q}`, body.Name)
	})

	loggedInTestServer(t, mux)

	err := FleetCreate([]string{"field trial"})

	if err != nil {
		t.Fatal(err)
	}

	if created != "field trial" {
		t.Errorf("the server saw %q created, want %q", created, "field trial")
	}
}

func TestFleetCreateRelaysARefusal(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /fleets", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "the body must carry a name", http.StatusBadRequest)
	})

	loggedInTestServer(t, mux)

	err := FleetCreate([]string{"field trial"})

	if err == nil || !strings.Contains(err.Error(), "the server said: the body must carry a name") {
		t.Fatalf("error = %v, want the relayed refusal", err)
	}
}

func TestFleetCreateTakesOneName(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
	}{
		{"no arguments", nil},
		{"two words", []string{"field", "trial"}},
		{"an empty name", []string{""}},
	}

	for _, test := range tests {
		err := FleetCreate(test.arguments)

		if err == nil || !strings.Contains(err.Error(), "takes one name") {
			t.Errorf("%s: error = %v, want the one-name hint", test.name, err)
		}
	}
}
