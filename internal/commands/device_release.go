package commands

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func DeviceRelease(arguments []string) error {
	if len(arguments) != 1 {
		return errors.New("device release takes an IMEI")
	}

	imei := arguments[0]

	if !validImei(imei) {
		return errors.New("the IMEI is the 15-digit number printed on the device")
	}

	devices, err := fetchDevices()

	if err != nil {
		return err
	}

	fleetId := int64(0)

	for _, device := range devices {
		if device.Imei == imei {
			fleetId = device.FleetId
		}
	}

	if fleetId == 0 {
		return errors.New("no such device, device list shows yours")
	}

	fleets, err := fetchFleets()

	if err != nil {
		return err
	}

	fleetName := ""

	for _, fleet := range fleets {
		if fleet.Id == fleetId {
			fleetName = fleet.Name
		}
	}

	if fleetName == "" {
		return errors.New("no such device, device list shows yours")
	}

	fmt.Printf("Release the device from %q? It can be claimed again afterwards. [y/N] ", fleetName)

	answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')

	answer = strings.ToLower(strings.TrimSpace(answer))

	if answer != "y" && answer != "yes" {
		fmt.Println("Nothing released.")
		return nil
	}

	request, err := authenticatedRequest(http.MethodDelete, "/devices/"+imei, nil)

	if err != nil {
		return err
	}

	response, err := apiClient.Do(request)

	if err != nil {
		return fmt.Errorf("the server could not be reached: %w", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusNoContent {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))

		return fmt.Errorf("the server said: %s", strings.TrimSpace(string(message)))
	}

	fmt.Println("Released the device.")

	return nil
}
