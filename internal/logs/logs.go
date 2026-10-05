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

const timeLayout = "2006-01-02T15:04:05-07:00"

func Tail(invocation api.Invocation, arguments []string) error {
	positionals := []string{}
	count := 10
	follow := true

	for index := 0; index < len(arguments); index++ {
		if arguments[index] != "-n" {
			positionals = append(positionals, arguments[index])
			continue
		}

		index++

		value := ""

		if index < len(arguments) {
			value = arguments[index]
		}

		parsed, err := strconv.Atoi(value)

		if err != nil || parsed < 0 {
			return errors.New("-n needs the number of logs to show")
		}

		count = parsed
		follow = false
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

	path := "/fleets/" + strconv.FormatInt(fleetId, 10) + "/logs?"
	query := url.Values{"imei": imeis, "last": {strconv.Itoa(count)}}
	failures := 0
	remaining := count

	for {
		request, err := api.AuthenticatedRequest(invocation, http.MethodGet, path+query.Encode(), nil)

		if err != nil {
			return err
		}

		answer := struct {
			Logs []struct {
				Imei       string    `json:"imei"`
				Name       string    `json:"name"`
				Kind       string    `json:"kind"`
				Text       string    `json:"text"`
				ReceivedAt time.Time `json:"received_at"`
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
				fmt.Fprintln(os.Stderr, time.Now().Format(timeLayout)+" superstack: The server stopped answering. Trying again.")
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
			fmt.Fprintln(os.Stderr, time.Now().Format(timeLayout)+" superstack: The server is answering again.")

			failures = 0
		}

		ended := false

		if !follow {
			isFull := len(answer.Logs) == 1000 // the most the server puts in one answer

			if len(answer.Logs) > remaining {
				answer.Logs = answer.Logs[:remaining]
			}

			remaining -= len(answer.Logs)
			ended = remaining == 0 || !isFull
		}

		lines := strings.Builder{}

		for _, entry := range answer.Logs {
			prefix := entry.ReceivedAt.Local().Format(timeLayout) + " " + entry.Imei + " " + entry.Kind + " [" + api.Printable(entry.Name) + "] "

			for _, line := range strings.Split(entry.Text, "\n") {
				lines.WriteString(prefix)

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

		fmt.Fprint(invocation.Out, lines.String())

		if ended {
			return nil
		}

		// The server holds this request for up to 20 seconds when it has no logs, inside the client's 30-second timeout.
		query = url.Values{"after": {strconv.FormatInt(answer.Next, 10)}, "imei": imeis}
	}
}
