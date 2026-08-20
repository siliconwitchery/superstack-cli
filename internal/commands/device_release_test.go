package commands

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

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

			session, out := loggedInSession(t, mux)
			session.In = strings.NewReader(test.answer)

			err := DeviceRelease(session, []string{"354820091234567"})

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
		err := DeviceRelease(Session{}, test.arguments)

		if err == nil || !strings.Contains(err.Error(), test.wantError) {
			t.Errorf("%s: error = %v", test.name, err)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /devices", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[]`)
	})
	session, _ := loggedInSession(t, mux)

	err := DeviceRelease(session, []string{"354820091234567"})

	if err == nil || err.Error() != "no such device, device list shows yours" {
		t.Fatalf("error = %v", err)
	}
}
