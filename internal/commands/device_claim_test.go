package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestDeviceClaim(t *testing.T) {
	claimedImei := ""
	claimedName := ""

	mux := http.NewServeMux()
	mux.HandleFunc("GET /fleets", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"id":3,"name":"pilot","owner":true}]`)
	})
	mux.HandleFunc("POST /fleets/{id}/devices", func(w http.ResponseWriter, r *http.Request) {
		body := struct {
			Imei string `json:"imei"`
			Name string `json:"name"`
		}{}

		json.NewDecoder(r.Body).Decode(&body)
		claimedImei = body.Imei
		claimedName = body.Name

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
		}

		w.WriteHeader(http.StatusNoContent)
	})

	loggedInTestServer(t, mux)

	printed, err := captureStdout(t, func() error {
		return DeviceClaim([]string{"354820091234567", "3", "roof sensor"})
	})

	if err != nil {
		t.Fatal(err)
	}

	if claimedImei != "354820091234567" || claimedName != "roof sensor" {
		t.Errorf("the server received IMEI %q and name %q", claimedImei, claimedName)
	}

	if printed != "Claimed the device into \"pilot\".\n" {
		t.Errorf("output = %q", printed)
	}
}

func TestDeviceClaimOmitsAnAbsentName(t *testing.T) {
	nameWasPresent := false

	mux := http.NewServeMux()
	mux.HandleFunc("GET /fleets", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"id":3,"name":"pilot","owner":true}]`)
	})
	mux.HandleFunc("POST /fleets/{id}/devices", func(w http.ResponseWriter, r *http.Request) {
		body := map[string]string{}
		json.NewDecoder(r.Body).Decode(&body)
		_, nameWasPresent = body["name"]
		w.WriteHeader(http.StatusNoContent)
	})

	loggedInTestServer(t, mux)

	err := DeviceClaim([]string{"354820091234567", "3"})

	if err != nil {
		t.Fatal(err)
	}

	if nameWasPresent {
		t.Error("the request included a name although none was given")
	}
}

func TestDeviceClaimServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /fleets", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"id":3,"name":"pilot","owner":true}]`)
	})
	mux.HandleFunc("POST /fleets/{id}/devices", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no unclaimed device with that IMEI", http.StatusNotFound)
	})

	loggedInTestServer(t, mux)

	err := DeviceClaim([]string{"354820091234567", "3"})

	if err == nil || err.Error() != "the server said: no unclaimed device with that IMEI" {
		t.Fatalf("error = %v", err)
	}
}

func TestDeviceClaimArguments(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
		wantError string
	}{
		{"no arguments", nil, "takes an IMEI"},
		{"too many arguments", []string{"354820091234567", "3", "one", "two"}, "takes an IMEI"},
		{"short IMEI", []string{"123", "3"}, "15-digit"},
		{"non-digit IMEI", []string{"35482009123456x", "3"}, "15-digit"},
		{"wordy fleet", []string{"354820091234567", "pilot"}, "shown by fleet list"},
		{"zero fleet", []string{"354820091234567", "0"}, "shown by fleet list"},
	}

	for _, test := range tests {
		err := DeviceClaim(test.arguments)

		if err == nil || !strings.Contains(err.Error(), test.wantError) {
			t.Errorf("%s: error = %v, want it to mention %q", test.name, err, test.wantError)
		}
	}
}

func TestDeviceClaimUnknownFleetUsesFleetIdGuidance(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /fleets", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[]`)
	})

	loggedInTestServer(t, mux)

	err := DeviceClaim([]string{"354820091234567", "9"})

	if err == nil || !strings.Contains(err.Error(), "shown by fleet list") {
		t.Fatalf("error = %v", err)
	}
}
