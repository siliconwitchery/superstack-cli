package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestDeviceClaim(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		message    string
		wantOutput string
		wantError  string
	}{
		{
			name:       "button pressed",
			statusCode: http.StatusNoContent,
			wantOutput: "Press the button on the device to finish claiming it.\nClaimed the device into \"pilot\".\n",
		},
		{
			name:       "button not pressed",
			statusCode: http.StatusRequestTimeout,
			message:    "the button was not pressed in time",
			wantOutput: "Press the button on the device to finish claiming it.\n",
			wantError:  "the server said: the button was not pressed in time",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
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

				if test.message != "" {
					http.Error(w, test.message, test.statusCode)

					return
				}

				w.WriteHeader(test.statusCode)
			})

			session, out := loggedInSession(t, mux)

			err := DeviceClaim(session, []string{"354820091234567", "3", "roof sensor"})

			printed := out.String()

			if test.wantError == "" && err != nil {
				t.Fatal(err)
			}

			if test.wantError != "" && (err == nil || err.Error() != test.wantError) {
				t.Fatalf("error = %v, want %q", err, test.wantError)
			}

			if claimedImei != "354820091234567" || claimedName != "roof sensor" {
				t.Errorf("the server received IMEI %q and name %q", claimedImei, claimedName)
			}

			if printed != test.wantOutput {
				t.Errorf("output = %q, want %q", printed, test.wantOutput)
			}
		})
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

	session, out := loggedInSession(t, mux)

	err := DeviceClaim(session, []string{"354820091234567", "3"})

	if err != nil {
		t.Fatal(err)
	}

	if nameWasPresent {
		t.Error("the request included a name although none was given")
	}

	if out.String() != "Press the button on the device to finish claiming it.\nClaimed the device into \"pilot\".\n" {
		t.Errorf("output = %q", out.String())
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
		err := DeviceClaim(Session{}, test.arguments)

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

	session, _ := loggedInSession(t, mux)

	err := DeviceClaim(session, []string{"354820091234567", "9"})

	if err == nil || !strings.Contains(err.Error(), "shown by fleet list") {
		t.Fatalf("error = %v", err)
	}
}
