package commands

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestDeviceRename(t *testing.T) {
	tests := []struct {
		name       string
		refusal    string
		wantOutput string
		wantError  string
	}{
		{name: "renamed", wantOutput: "Renamed the device to \"pilot\".\n"},
		{name: "server refusal", refusal: "no such device", wantError: "no such device"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
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

				if test.refusal != "" {
					http.Error(w, test.refusal, http.StatusNotFound)
					return
				}

				w.WriteHeader(http.StatusNoContent)
			})

			session, out := loggedInSession(t, mux)

			err := DeviceRename(session, []string{"354820091234567", " pilot "})

			if test.wantError != "" {
				if err == nil || err.Error() != test.wantError {
					t.Fatalf("error = %v, want %q", err, test.wantError)
				}
			} else if err != nil {
				t.Fatal(err)
			}

			if renamedPath != "/devices/354820091234567" || renamedTo != "pilot" {
				t.Errorf("the server saw %q renamed to %q", renamedPath, renamedTo)
			}

			if out.String() != test.wantOutput {
				t.Errorf("output = %q, want %q", out.String(), test.wantOutput)
			}
		})
	}
}

func TestDeviceRenameArguments(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
		wantError string
	}{
		{"no arguments", nil, "takes an IMEI and a new name"},
		{"one argument", []string{"354820091234567"}, "takes an IMEI and a new name"},
		{"three arguments", []string{"354820091234567", "roof", "sensor"}, "takes an IMEI and a new name"},
		{"short IMEI", []string{"123", "pilot"}, "15-digit"},
		{"non-digit IMEI", []string{"35482009123456x", "pilot"}, "15-digit"},
		{"empty name", []string{"354820091234567", ""}, "takes an IMEI and a new name"},
		{"whitespace name", []string{"354820091234567", "  "}, "takes an IMEI and a new name"},
	}

	for _, test := range tests {
		err := DeviceRename(Session{}, test.arguments)

		if err == nil || !strings.Contains(err.Error(), test.wantError) {
			t.Errorf("%s: error = %v, want it to mention %q", test.name, err, test.wantError)
		}
	}
}
