package logs

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/siliconwitchery/superstack-cli/internal/api"
)

func Tail(invocation api.Invocation, arguments []string) error {
	positionals := []string{}
	count := 10
	logFilePath := ""

	for index := 0; index < len(arguments); index++ {
		switch arguments[index] {
		case "-n":
			index++

			value := ""

			if index < len(arguments) {
				value = arguments[index]
			}

			parsed, err := strconv.Atoi(value)

			if err != nil || parsed < 0 || parsed > 1000 {
				return errors.New("-n needs the number of earlier logs to show, from 0 to 1000")
			}

			count = parsed

		case "--log-file":
			index++

			if index == len(arguments) {
				return errors.New("--log-file needs a file")
			}

			logFilePath = arguments[index]

		default:
			positionals = append(positionals, arguments[index])
		}
	}

	if len(positionals) == 0 {
		return errors.New("tail takes one fleet id, then optional IMEIs, each the 15-digit number printed on the device")
	}

	imeis := []string{}

	for index, positional := range positionals {
		isImei := len(positional) == 15 && !strings.ContainsFunc(positional, func(digit rune) bool { return digit < '0' || digit > '9' })

		if index == 0 && isImei || index > 0 && !isImei {
			return errors.New("tail takes one fleet id, then optional IMEIs, each the 15-digit number printed on the device")
		}

		if isImei {
			imeis = append(imeis, positional)
		}
	}

	fleetId, err := strconv.ParseInt(positionals[0], 10, 64)

	if err != nil || fleetId < 1 {
		return errors.New("the fleet id is the number shown by fleet list")
	}

	var logFile *os.File

	if logFilePath != "" {
		logFile, err = os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)

		if err != nil {
			return fmt.Errorf("%s could not be opened for writing", logFilePath)
		}

		defer logFile.Close()
	}

	show := func(lines string) error {
		fmt.Fprint(invocation.Out, lines)

		if logFile == nil {
			return nil
		}

		_, err := logFile.WriteString(lines)

		if err != nil {
			return fmt.Errorf("%s could not be written", logFilePath)
		}

		return nil
	}

	path := "/fleets/" + strconv.FormatInt(fleetId, 10) + "/logs?"
	query := url.Values{"imei": imeis, "last": {strconv.Itoa(count)}}
	failures := 0

	for {
		request, err := api.AuthenticatedRequest(invocation, http.MethodGet, path+query.Encode(), nil)

		if err != nil {
			return err
		}

		answer := struct {
			Logs []struct {
				Imei       string  `json:"imei"`
				Name       *string `json:"name"`
				Kind       string  `json:"kind"`
				Text       string  `json:"text"`
				ReceivedAt string  `json:"received_at"`
			} `json:"logs"`
			Next int64 `json:"next"`
		}{}

		response, err := invocation.Client.Do(request)

		failed := err != nil

		if !failed {
			switch {
			case response.StatusCode == http.StatusOK:
				err = api.Decode(response, &answer)

				failed = err != nil

			case response.StatusCode >= 500:
				failed = true

			default:
				refusal := api.ServerError(response)

				response.Body.Close()

				return refusal
			}

			response.Body.Close()
		}

		// The query is unchanged on a retry, so no log is lost or shown twice.
		if failed {
			if failures == 0 {
				err = show(time.Now().Format("2006-01-02 15:04:05") + " superstack: The server stopped answering. Trying again.\n")

				if err != nil {
					return err
				}
			}

			delay := 10 * time.Second

			if failures < 3 {
				delay = time.Second << failures // 1, 2, then 4 seconds
			}

			time.Sleep(delay)

			failures++

			continue
		}

		if failures > 0 {
			err = show(time.Now().Format("2006-01-02 15:04:05") + " superstack: The server is answering again.\n")

			if err != nil {
				return err
			}

			failures = 0
		}

		lines := strings.Builder{}

		for _, entry := range answer.Logs {
			received := "---------- --:--:--"

			receivedAt, err := time.Parse(time.RFC3339, entry.ReceivedAt)

			if err == nil {
				received = receivedAt.Local().Format("2006-01-02 15:04:05")
			}

			device := entry.Imei

			if entry.Name != nil && *entry.Name != "" {
				device = *entry.Name
			}

			for _, line := range strings.Split(entry.Text, "\n") {
				lines.WriteString(received + " " + api.Printable(device) + "[" + api.Printable(entry.Kind) + "]: ")

				// Tabs and every graphic character print as Lua would print
				// them. The rest is escaped so a device cannot drive the terminal.
				for _, letter := range line {
					if letter == '\t' || strconv.IsGraphic(letter) {
						lines.WriteRune(letter)
						continue
					}

					quoted := strconv.QuoteRuneToGraphic(letter)

					lines.WriteString(quoted[1 : len(quoted)-1])
				}

				lines.WriteString("\n")
			}
		}

		err = show(lines.String())

		if err != nil {
			return err
		}

		// The server holds this request for up to 20 seconds, inside the client's 30-second timeout.
		query = url.Values{"after": {strconv.FormatInt(answer.Next, 10)}, "imei": imeis, "wait": {"20"}}
	}
}
