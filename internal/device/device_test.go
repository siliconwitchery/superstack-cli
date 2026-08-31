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

func TestDeviceList(t *testing.T) {
	now := time.Now()
	devices := fmt.Sprintf(`[{"imei":"111111111111111","name":"roof","fleet_id":3,"last_seen_at":%q,"run_state":"running","storage_used":128,"storage_total":1024},`+
		`{"imei":"222222222222222","name":null,"fleet_id":4,"last_seen_at":%q,"run_state":"crashed","storage_used":1536,"storage_total":1048576},`+
		`{"imei":"333333333333333","name":"shed","fleet_id":3,"last_seen_at":null,"run_state":null,"storage_used":null,"storage_total":null}]`,
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
		{name: "table", wantShown: []string{"IMEI             NAME  FLEET", "RUN STATE", "STORAGE", "LAST SEEN", "roof", "pilot", "running", "128 B / 1.0 KiB", "workshop", "crashed", "1.5 KiB / 1.0 MiB", "3 h ago", "never"}},
		{name: "filtered", arguments: []string{"3"}, wantShown: []string{"111111111111111", "333333333333333"}, wantHidden: []string{"222222222222222", "workshop"}},
		{name: "json flag anywhere", arguments: []string{"3", "--json"}, wantShown: []string{`"imei":"111111111111111"`, `"fleet_id":3`, `"last_seen_at":`, `"run_state":"running"`, `"storage_used":128`, `"storage_total":1024`}, wantHidden: []string{"LAST SEEN", "222222222222222", `"reported_state"`}},
		{name: "empty fleet", arguments: []string{"5"}, wantExact: "No devices in that fleet.\n"},
		{name: "no devices", devices: `[]`, fleets: `[]`, wantExact: "No devices yet.\n"},
		{name: "server refusal", refusal: "devices unavailable", wantError: "devices unavailable"},
		{name: "unknown fleet", arguments: []string{"9"}, wantError: "no such fleet"},
		{name: "two ids", arguments: []string{"3", "4"}, wantError: "takes at most one fleet id"},
		{name: "wordy id", arguments: []string{"pilot"}, wantError: "shown by fleet list"},
		{name: "an unreadable last seen time leaves the rest of the table", devices: `[{"imei":"111111111111111","name":"roof","fleet_id":3,"last_seen_at":"yesterday","run_state":"stopped","storage_used":0,"storage_total":1024}]`, wantShown: []string{"111111111111111  roof  pilot  stopped", "0 B / 1.0 KiB", "unknown"}},
		{name: "a fleet the list does not name", devices: `[{"imei":"888888888888888","name":"orphan","fleet_id":99}]`, wantShown: []string{"888888888888888  orphan  -", "never"}},
		{name: "a name with control characters is escaped", devices: `[{"imei":"111111111111111","name":"\u001b[2K\rhidden","fleet_id":3}]`, wantShown: []string{`\x1b[2K\rhidden`}, wantHidden: []string{"\x1b"}},
		{name: "minutes ago", devices: fmt.Sprintf(`[{"imei":"111111111111111","name":"roof","fleet_id":3,"last_seen_at":%q}]`, now.Add(-12*time.Minute).Format(time.RFC3339)), wantShown: []string{"12 min ago"}},
		{name: "days ago", devices: fmt.Sprintf(`[{"imei":"111111111111111","name":"roof","fleet_id":3,"last_seen_at":%q}]`, now.Add(-49*time.Hour).Format(time.RFC3339)), wantShown: []string{"2 d ago"}},
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
			invocation, out := apitest.LoggedInInvocation(t, mux)

			err := List(invocation, test.arguments)

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

func TestDevicePair(t *testing.T) {
	tests := []struct {
		name       string
		arguments  []string
		wantBody   string
		wantOutput string
	}{
		{
			name:       "without a name",
			arguments:  []string{"354820091234567", "3"},
			wantBody:   `{"imei":"354820091234567"}`,
			wantOutput: "Press the pairing button on device \"354820091234567\".\nPaired device \"354820091234567\" with fleet 3.\n",
		},
		{
			name:       "with a name",
			arguments:  []string{"354820091234567", "3", "  rooftop  "},
			wantBody:   `{"imei":"354820091234567","name":"rooftop"}`,
			wantOutput: "Press the pairing button on device \"rooftop\".\nPaired device \"rooftop\" with fleet 3.\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := ""
			mux := http.NewServeMux()
			mux.HandleFunc("POST /fleets/3/devices", func(w http.ResponseWriter, r *http.Request) {
				decoded := map[string]any{}

				if err := json.NewDecoder(r.Body).Decode(&decoded); err != nil {
					t.Fatal(err)
				}

				encoded, err := json.Marshal(decoded)

				if err != nil {
					t.Fatal(err)
				}

				body = string(encoded)
				w.WriteHeader(http.StatusNoContent)
			})

			invocation, out := apitest.LoggedInInvocation(t, mux)

			err := Pair(invocation, test.arguments)

			if err != nil {
				t.Fatal(err)
			}

			if body != test.wantBody {
				t.Errorf("body = %q, want %q", body, test.wantBody)
			}

			if out.String() != test.wantOutput {
				t.Errorf("output = %q, want %q", out.String(), test.wantOutput)
			}
		})
	}
}

func TestDevicePairArgumentsAndRefusal(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
		wantError string
	}{
		{"no arguments", nil, "takes an IMEI"},
		{"one argument", []string{"354820091234567"}, "takes an IMEI"},
		{"four arguments", []string{"354820091234567", "3", "roof", "extra"}, "takes an IMEI"},
		{"short IMEI", []string{"123", "3"}, "15-digit"},
		{"non-digit IMEI", []string{"35482009123456x", "3"}, "15-digit"},
		{"zero fleet id", []string{"354820091234567", "0"}, "fleet id"},
		{"unreadable fleet id", []string{"354820091234567", "crew"}, "fleet id"},
		{"empty name", []string{"354820091234567", "3", "  "}, "cannot be empty"},
	}

	for _, test := range tests {
		err := Pair(api.Invocation{}, test.arguments)

		if err == nil || !strings.Contains(err.Error(), test.wantError) {
			t.Errorf("%s: error = %v, want it to mention %q", test.name, err, test.wantError)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /fleets/3/devices", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no such device", http.StatusNotFound)
	})

	invocation, _ := apitest.LoggedInInvocation(t, mux)

	err := Pair(invocation, []string{"354820091234567", "3"})

	if err == nil || err.Error() != "no such device" {
		t.Fatalf("error = %v, want no such device", err)
	}
}

func TestDeviceRename(t *testing.T) {
	tests := []struct {
		name       string
		refusal    string
		wantOutput string
		wantError  string
	}{
		{name: "renamed", wantOutput: "Renamed device 354820091234567 to \"pilot\".\n"},
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

			invocation, out := apitest.LoggedInInvocation(t, mux)

			err := Rename(invocation, []string{"354820091234567", " pilot "})

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
		err := Rename(api.Invocation{}, test.arguments)

		if err == nil || !strings.Contains(err.Error(), test.wantError) {
			t.Errorf("%s: error = %v, want it to mention %q", test.name, err, test.wantError)
		}
	}
}

func TestDeviceUnpair(t *testing.T) {
	tests := []struct {
		name         string
		answer       string
		devices      string
		fleets       string
		refusal      string
		wantUnpaired bool
		wantOutput   string
		wantError    string
	}{
		{name: "confirmed", answer: "yes\n", wantUnpaired: true, wantOutput: "Unpair device \"354820091234567\" from fleet \"pilot\"? It will no longer appear in the fleet. [y/N] Unpaired device \"354820091234567\" from fleet \"pilot\".\n"},
		{name: "declined", answer: "n\n", wantOutput: "Unpair device \"354820091234567\" from fleet \"pilot\"? It will no longer appear in the fleet. [y/N] Nothing unpaired.\n"},
		{name: "a named device is named back, not its IMEI", answer: "n\n", devices: `[{"imei":"354820091234567","name":"rooftop","fleet_id":3,"last_seen_at":null}]`, wantOutput: "Unpair device \"rooftop\" from fleet \"pilot\"? It will no longer appear in the fleet. [y/N] Nothing unpaired.\n"},
		{name: "server refuses", answer: "y\n", refusal: "no such device", wantUnpaired: true, wantError: "no such device"},
		{name: "device belongs to an inaccessible fleet", fleets: `[]`, wantError: "no such device, device list shows yours"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			unpairedPath := ""
			fleets := test.fleets
			devices := test.devices

			if fleets == "" {
				fleets = `[{"id":3,"name":"pilot","owner":true}]`
			}

			if devices == "" {
				devices = `[{"imei":"354820091234567","name":null,"fleet_id":3,"last_seen_at":null}]`
			}

			mux := http.NewServeMux()
			mux.HandleFunc("GET /devices", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, devices)
			})
			mux.HandleFunc("GET /fleets", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, fleets)
			})
			mux.HandleFunc("DELETE /devices/{imei}", func(w http.ResponseWriter, r *http.Request) {
				unpairedPath = r.URL.Path

				if test.refusal != "" {
					http.Error(w, test.refusal, http.StatusNotFound)
					return
				}

				w.WriteHeader(http.StatusNoContent)
			})

			invocation, out := apitest.LoggedInInvocation(t, mux)
			invocation.In = strings.NewReader(test.answer)

			err := Unpair(invocation, []string{"354820091234567"})

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

			if test.wantUnpaired && unpairedPath != "/devices/354820091234567" {
				t.Errorf("unpaired path = %q", unpairedPath)
			}

			if !test.wantUnpaired && unpairedPath != "" {
				t.Errorf("unpaired path = %q after decline", unpairedPath)
			}
		})
	}
}

func TestDeviceUnpairArgumentsAndUnknownDevice(t *testing.T) {
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
		err := Unpair(api.Invocation{}, test.arguments)

		if err == nil || !strings.Contains(err.Error(), test.wantError) {
			t.Errorf("%s: error = %v", test.name, err)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /devices", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[]`)
	})
	invocation, _ := apitest.LoggedInInvocation(t, mux)

	err := Unpair(invocation, []string{"354820091234567"})

	if err == nil || err.Error() != "no such device, device list shows yours" {
		t.Fatalf("error = %v", err)
	}
}
