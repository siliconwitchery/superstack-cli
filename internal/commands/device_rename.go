package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

func DeviceRename(session Session, arguments []string) error {
	if len(arguments) != 2 {
		return errors.New("device rename takes an IMEI and a name, quoted if it has spaces")
	}

	imei := arguments[0]

	if !validImei(imei) {
		return errors.New("the IMEI is the 15-digit number printed on the device")
	}

	name := strings.TrimSpace(arguments[1])

	if name == "" {
		return errors.New("device rename takes an IMEI and a name, quoted if it has spaces")
	}

	body, err := json.Marshal(map[string]string{"name": name})

	if err != nil {
		return err
	}

	request, err := authenticatedRequest(session, http.MethodPatch, "/devices/"+imei, bytes.NewReader(body))

	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := session.Client.Do(request)

	if err != nil {
		return fmt.Errorf("the server could not be reached: %w", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusNoContent {
		return serverError(response)
	}

	fmt.Fprintf(session.Out, "Renamed the device to %q.\n", name)

	return nil
}
