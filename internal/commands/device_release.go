package commands

import (
	"bufio"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

func DeviceRelease(session Session, arguments []string) error {
	if len(arguments) != 1 {
		return errors.New("device release takes an IMEI")
	}

	imei := arguments[0]

	if !validImei(imei) {
		return errors.New("the IMEI is the 15-digit number printed on the device")
	}

	devices, err := fetchDevices(session)

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

	fleets, err := fetchFleets(session)

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

	fmt.Fprintf(session.Out, "Release the device from %q? It erases everything on the device, and claiming it again means pressing its pairing button in person. [y/N] ", fleetName)

	answer, _ := bufio.NewReader(session.In).ReadString('\n')

	answer = strings.ToLower(strings.TrimSpace(answer))

	if answer != "y" && answer != "yes" {
		fmt.Fprintln(session.Out, "Nothing released.")
		return nil
	}

	request, err := authenticatedRequest(session, http.MethodDelete, "/devices/"+imei, nil)

	if err != nil {
		return err
	}

	response, err := session.Client.Do(request)

	if err != nil {
		return errors.New("the server could not be reached, check your connection")
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusNoContent {
		return serverError(response)
	}

	fmt.Fprintln(session.Out, "Released the device.")

	return nil
}
