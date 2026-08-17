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
	devices := fmt.Sprintf(`[{"imei":"111111111111111","name":"roof","fleet_id":3,"last_seen_at":%q},`+
		`{"imei":"222222222222222","name":null,"fleet_id":4,"last_seen_at":%q},`+
		`{"imei":"333333333333333","name":"shed","fleet_id":3,"last_seen_at":null}]`,
		now.Add(-time.Minute).Format(time.RFC3339), now.Add(-3*time.Hour).Format(time.RFC3339))
	fleets := `[{"id":3,"name":"pilot","owner":true},{"id":4,"name":"workshop","owner":true},{"id":5,"name":"empty","owner":true}]`

	tests := []struct {
		name       string
		arguments  []string
		wantShown  []string
		wantHidden []string
		wantExact  string
		wantError  string
	}{
		{"table", nil, []string{"IMEI             NAME  FLEET     LAST SEEN", "roof", "pilot", "just now", "-", "workshop", "3 h ago", "never"}, nil, "", ""},
		{"filtered", []string{"3"}, []string{"111111111111111", "333333333333333"}, []string{"222222222222222", "workshop"}, "", ""},
		{"json flag anywhere", []string{"3", "--json"}, []string{`"imei":"111111111111111"`, `"fleet_id":3`}, []string{"LAST SEEN", "222222222222222"}, "", ""},
		{"empty fleet", []string{"5"}, nil, nil, "No devices in that fleet.\n", ""},
		{"unknown fleet", []string{"9"}, nil, nil, "", "no such fleet"},
		{"two ids", []string{"3", "4"}, nil, nil, "", "takes at most one fleet id"},
		{"wordy id", []string{"pilot"}, nil, nil, "", "shown by fleet list"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("GET /devices", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, devices) })
			mux.HandleFunc("GET /fleets", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, fleets) })
			loggedInTestServer(t, mux)

			printed, err := captureStdout(t, func() error { return DeviceList(test.arguments) })

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

func TestDeviceListEmptyAndServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /devices", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `[]`) })
	mux.HandleFunc("GET /fleets", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `[]`) })
	loggedInTestServer(t, mux)

	printed, err := captureStdout(t, func() error { return DeviceList(nil) })

	if err != nil {
		t.Fatal(err)
	}

	if printed != "No devices yet. Claim one with device claim.\n" {
		t.Errorf("output = %q", printed)
	}

	errorMux := http.NewServeMux()
	errorMux.HandleFunc("GET /devices", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "devices unavailable", http.StatusServiceUnavailable)
	})
	loggedInTestServer(t, errorMux)

	err = DeviceList(nil)

	if err == nil || err.Error() != "the server said: devices unavailable" {
		t.Fatalf("error = %v", err)
	}
}
