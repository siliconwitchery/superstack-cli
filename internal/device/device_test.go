package device

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/siliconwitchery/superstack-cli/internal/api"
	"github.com/siliconwitchery/superstack-cli/internal/api/apitest"
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
			wantOutput: "Press the pairing button on the device to finish claiming it.\nClaimed the device into \"pilot\".\n",
		},
		{
			name:       "button not pressed",
			statusCode: http.StatusRequestTimeout,
			message:    "the button was not pressed in time",
			wantOutput: "Press the pairing button on the device to finish claiming it.\n",
			wantError:  "the button was not pressed in time",
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

			session, out := apitest.LoggedInSession(t, mux)

			err := Claim(session, []string{"354820091234567", "3", "roof sensor"})

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

	session, out := apitest.LoggedInSession(t, mux)

	err := Claim(session, []string{"354820091234567", "3"})

	if err != nil {
		t.Fatal(err)
	}

	if nameWasPresent {
		t.Error("the request included a name although none was given")
	}

	if out.String() != "Press the pairing button on the device to finish claiming it.\nClaimed the device into \"pilot\".\n" {
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
		err := Claim(api.Session{}, test.arguments)

		if err == nil || !strings.Contains(err.Error(), test.wantError) {
			t.Errorf("%s: error = %v, want it to mention %q", test.name, err, test.wantError)
		}
	}
}

func TestDeviceClaimUnknownFleet(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /fleets", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[]`)
	})

	session, _ := apitest.LoggedInSession(t, mux)

	err := Claim(session, []string{"354820091234567", "9"})

	if err == nil || err.Error() != "no such fleet" {
		t.Fatalf("error = %v", err)
	}
}

func TestDeviceList(t *testing.T) {
	now := time.Now()
	devices := fmt.Sprintf(`[{"imei":"111111111111111","name":"roof","fleet_id":3,"last_seen_at":%q,"reported_state":2,"storage_used":1240,"storage_total":57344},`+
		`{"imei":"222222222222222","name":null,"fleet_id":4,"last_seen_at":%q,"reported_state":4,"storage_used":2500000,"storage_total":8000000},`+
		`{"imei":"333333333333333","name":"shed","fleet_id":3,"last_seen_at":null,"reported_state":null,"storage_used":null,"storage_total":null}]`,
		now.Add(-time.Minute).Format(time.RFC3339), now.Add(-3*time.Hour).Format(time.RFC3339))
	fleets := `[{"id":3,"name":"pilot","owner":true},{"id":4,"name":"workshop","owner":true},{"id":5,"name":"empty","owner":true}]`

	tests := []struct {
		name       string
		arguments  []string
		wantShown  []string
		wantHidden []string
		wantExact  string
		wantError  string
		devices    string
		fleets     string
		refusal    string
	}{
		{name: "table", wantShown: []string{"IMEI             NAME  FLEET     STATE    STORAGE            LAST SEEN", "roof", "pilot", "running", "1.2 kB of 57.3 kB", "just now", "-", "workshop", "crashed", "2.5 MB of 8.0 MB", "3 h ago", "unknown", "never"}},
		{name: "filtered", arguments: []string{"3"}, wantShown: []string{"111111111111111", "333333333333333"}, wantHidden: []string{"222222222222222", "workshop"}},
		{name: "json flag anywhere", arguments: []string{"3", "--json"}, wantShown: []string{`"imei":"111111111111111"`, `"fleet_id":3`}, wantHidden: []string{"LAST SEEN", "222222222222222"}},
		{name: "empty fleet", arguments: []string{"5"}, wantExact: "No devices in that fleet.\n"},
		{name: "no devices", devices: `[]`, fleets: `[]`, wantExact: "No devices yet. Claim one with device claim.\n"},
		{name: "server refusal", refusal: "devices unavailable", wantError: "devices unavailable"},
		{name: "unknown fleet", arguments: []string{"9"}, wantError: "no such fleet"},
		{name: "two ids", arguments: []string{"3", "4"}, wantError: "takes at most one fleet id"},
		{name: "wordy id", arguments: []string{"pilot"}, wantError: "shown by fleet list"},
		{name: "bad last seen time", devices: `[{"imei":"111111111111111","name":"roof","fleet_id":3,"last_seen_at":"yesterday"}]`, wantError: `cannot parse "yesterday"`},
		{name: "minutes ago", devices: fmt.Sprintf(`[{"imei":"111111111111111","name":"roof","fleet_id":3,"last_seen_at":%q}]`, now.Add(-12*time.Minute).Format(time.RFC3339)), wantShown: []string{"12 min ago"}},
		{name: "days ago", devices: fmt.Sprintf(`[{"imei":"111111111111111","name":"roof","fleet_id":3,"last_seen_at":%q}]`, now.Add(-49*time.Hour).Format(time.RFC3339)), wantShown: []string{"2 d ago"}},
		{name: "stopped and undefined states", devices: `[{"imei":"444444444444444","name":"halted","fleet_id":3,"reported_state":3},{"imei":"555555555555555","name":"odd","fleet_id":3,"reported_state":1}]`, wantShown: []string{"stopped", "unknown"}},
		{name: "byte storage", devices: `[{"imei":"666666666666666","name":"bytes","fleet_id":3,"storage_used":999,"storage_total":999}]`, wantShown: []string{"999 B of 999 B"}},
		{name: "missing used storage", devices: `[{"imei":"777777777777777","name":"nil-used","fleet_id":3,"storage_used":null,"storage_total":57344}]`, wantShown: []string{"777777777777777  nil-used  pilot  unknown  -        never"}},
		{name: "missing total storage", devices: `[{"imei":"888888888888888","name":"nil-total","fleet_id":3,"storage_used":1240,"storage_total":null}]`, wantShown: []string{"888888888888888  nil-total  pilot  unknown  -        never"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			servedDevices := test.devices

			if servedDevices == "" {
				servedDevices = devices
			}

			servedFleets := test.fleets

			if servedFleets == "" {
				servedFleets = fleets
			}

			mux := http.NewServeMux()
			mux.HandleFunc("GET /devices", func(w http.ResponseWriter, r *http.Request) {
				if test.refusal != "" {
					http.Error(w, test.refusal, http.StatusServiceUnavailable)
					return
				}

				fmt.Fprint(w, servedDevices)
			})
			mux.HandleFunc("GET /fleets", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, servedFleets) })
			session, out := apitest.LoggedInSession(t, mux)

			err := List(session, test.arguments)

			printed := out.String()

			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error = %v", err)
				}
				return
			}

			if err != nil {
				t.Fatal(err)
			}

			if test.wantExact != "" && printed != test.wantExact {
				t.Errorf("output = %q", printed)
			}

			for _, want := range test.wantShown {
				if !strings.Contains(printed, want) {
					t.Errorf("output %q omits %q", printed, want)
				}
			}

			for _, hidden := range test.wantHidden {
				if strings.Contains(printed, hidden) {
					t.Errorf("output %q includes %q", printed, hidden)
				}
			}
		})
	}
}

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

			session, out := apitest.LoggedInSession(t, mux)

			err := Rename(session, []string{"354820091234567", " pilot "})

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
		err := Rename(api.Session{}, test.arguments)

		if err == nil || !strings.Contains(err.Error(), test.wantError) {
			t.Errorf("%s: error = %v, want it to mention %q", test.name, err, test.wantError)
		}
	}
}

func TestDeviceRelease(t *testing.T) {
	tests := []struct {
		name         string
		answer       string
		fleets       string
		refusal      string
		wantReleased bool
		wantOutput   string
		wantError    string
	}{
		{name: "confirmed", answer: "yes\n", wantReleased: true, wantOutput: "Release the device from \"pilot\"? It erases everything on the device, and claiming it again means pressing its pairing button in person. [y/N] Released the device.\n"},
		{name: "declined", answer: "n\n", wantOutput: "Release the device from \"pilot\"? It erases everything on the device, and claiming it again means pressing its pairing button in person. [y/N] Nothing released.\n"},
		{name: "server refuses", answer: "y\n", refusal: "no such device", wantReleased: true, wantError: "no such device"},
		{name: "device belongs to an inaccessible fleet", fleets: `[]`, wantError: "no such device, device list shows yours"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			releasedPath := ""
			fleets := test.fleets

			if fleets == "" {
				fleets = `[{"id":3,"name":"pilot","owner":true}]`
			}

			mux := http.NewServeMux()
			mux.HandleFunc("GET /devices", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `[{"imei":"354820091234567","name":null,"fleet_id":3,"last_seen_at":null}]`)
			})
			mux.HandleFunc("GET /fleets", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, fleets)
			})
			mux.HandleFunc("DELETE /devices/{imei}", func(w http.ResponseWriter, r *http.Request) {
				releasedPath = r.URL.Path

				if test.refusal != "" {
					http.Error(w, test.refusal, http.StatusNotFound)
					return
				}

				w.WriteHeader(http.StatusNoContent)
			})

			session, out := apitest.LoggedInSession(t, mux)
			session.In = strings.NewReader(test.answer)

			err := Release(session, []string{"354820091234567"})

			printed := out.String()

			if test.wantError != "" {
				if err == nil || err.Error() != test.wantError {
					t.Fatalf("error = %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}

			if test.wantOutput != "" && printed != test.wantOutput {
				t.Errorf("output = %q", printed)
			}

			if test.wantReleased && releasedPath != "/devices/354820091234567" {
				t.Errorf("released path = %q", releasedPath)
			}

			if !test.wantReleased && releasedPath != "" {
				t.Errorf("released path = %q after decline", releasedPath)
			}
		})
	}
}

func TestDeviceReleaseArgumentsAndUnknownDevice(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
		wantError string
	}{
		{"no arguments", nil, "takes an IMEI"},
		{"two arguments", []string{"354820091234567", "extra"}, "takes an IMEI"},
		{"short IMEI", []string{"123"}, "15-digit"},
		{"non-digit IMEI", []string{"35482009123456x"}, "15-digit"},
	}

	for _, test := range tests {
		err := Release(api.Session{}, test.arguments)

		if err == nil || !strings.Contains(err.Error(), test.wantError) {
			t.Errorf("%s: error = %v", test.name, err)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /devices", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[]`)
	})
	session, _ := apitest.LoggedInSession(t, mux)

	err := Release(session, []string{"354820091234567"})

	if err == nil || err.Error() != "no such device, device list shows yours" {
		t.Fatalf("error = %v", err)
	}
}
