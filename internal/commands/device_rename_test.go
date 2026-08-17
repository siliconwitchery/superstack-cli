package commands

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestDeviceRename(t *testing.T) {
	renamedPath := ""
	renamedTo := ""

	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /devices/{imei}", func(w http.ResponseWriter, r *http.Request) {
		body := struct {
			Name string `json:"name"`
		}{}

		json.NewDecoder(r.Body).Decode(&body)
		renamedPath = r.URL.Path
		renamedTo = body.Name
		w.WriteHeader(http.StatusNoContent)
	})

	loggedInTestServer(t, mux)

	printed, err := captureStdout(t, func() error {
		return DeviceRename([]string{"354820091234567", " pilot "})
	})

	if err != nil {
		t.Fatal(err)
	}

	if renamedPath != "/devices/354820091234567" || renamedTo != "pilot" {
		t.Errorf("the server saw %q renamed to %q", renamedPath, renamedTo)
	}

	if printed != "Renamed the device to \"pilot\".\n" {
		t.Errorf("output = %q", printed)
	}
}

func TestDeviceRenameServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /devices/{imei}", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no such device", http.StatusNotFound)
	})

	loggedInTestServer(t, mux)

	err := DeviceRename([]string{"354820091234567", "pilot"})

	if err == nil || err.Error() != "the server said: no such device" {
		t.Fatalf("error = %v", err)
	}
}

func TestDeviceRenameArguments(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
		wantError string
	}{
		{"no arguments", nil, "takes an IMEI and a name"},
		{"one argument", []string{"354820091234567"}, "takes an IMEI and a name"},
		{"three arguments", []string{"354820091234567", "roof", "sensor"}, "takes an IMEI and a name"},
		{"short IMEI", []string{"123", "pilot"}, "15-digit"},
		{"non-digit IMEI", []string{"35482009123456x", "pilot"}, "15-digit"},
		{"empty name", []string{"354820091234567", ""}, "takes an IMEI and a name"},
		{"whitespace name", []string{"354820091234567", "  "}, "takes an IMEI and a name"},
	}

	for _, test := range tests {
		err := DeviceRename(test.arguments)

		if err == nil || !strings.Contains(err.Error(), test.wantError) {
			t.Errorf("%s: error = %v, want it to mention %q", test.name, err, test.wantError)
		}
	}
}
