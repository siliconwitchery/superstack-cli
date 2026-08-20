package commands

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

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
			session, out := loggedInSession(t, mux)

			err := DeviceList(session, test.arguments)

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

func TestFormatRunState(t *testing.T) {
	running := 2
	stopped := 3
	crashed := 4
	undefined := 1
	tests := []struct {
		name  string
		state *int
		want  string
	}{
		{"running", &running, "running"},
		{"stopped", &stopped, "stopped"},
		{"crashed", &crashed, "crashed"},
		{"undefined", &undefined, "unknown"},
		{"nil", nil, "unknown"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := formatRunState(test.state); got != test.want {
				t.Errorf("formatRunState() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestFormatStorage(t *testing.T) {
	bytes := int64(999)
	kilobytes := int64(1240)
	megabytes := int64(2500000)
	total := int64(57344)
	tests := []struct {
		name  string
		used  *int64
		total *int64
		want  string
	}{
		{"bytes", &bytes, &bytes, "999 B of 999 B"},
		{"kilobytes", &kilobytes, &total, "1.2 kB of 57.3 kB"},
		{"megabytes", &megabytes, &megabytes, "2.5 MB of 2.5 MB"},
		{"nil used", nil, &total, "-"},
		{"nil total", &kilobytes, nil, "-"},
		{"both nil", nil, nil, "-"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := formatStorage(test.used, test.total); got != test.want {
				t.Errorf("formatStorage() = %q, want %q", got, test.want)
			}
		})
	}
}
