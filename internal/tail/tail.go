package tail

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/siliconwitchery/superstack-cli/internal/api"
)

func Tail(invocation api.Invocation, arguments []string) error {
	positionals := []string{}
	count := int64(0)
	offset := int64(0)

	for index := 0; index < len(arguments); index++ {
		switch arguments[index] {
		case "-n", "-o":
			flag := arguments[index]

			if index+1 == len(arguments) {
				return fmt.Errorf("%s needs a number", flag)
			}

			index++

			value, err := strconv.ParseInt(arguments[index], 10, 64)

			if flag == "-n" {
				if err != nil || value < 1 || value > 1000 {
					return errors.New("-n takes a number of logs from 1 to 1000")
				}

				count = value
			} else {
				if err != nil || value < 1 {
					return errors.New("-o takes how many of the newest logs to skip, 1 or more")
				}

				offset = value
			}

		default:
			positionals = append(positionals, arguments[index])
		}
	}

	if len(positionals) == 0 {
		return errors.New("tail takes a fleet id, then the IMEIs of the devices to show if not all of them")
	}

	fleetID, err := strconv.ParseInt(positionals[0], 10, 64)

	if err != nil || fleetID < 1 {
		return errors.New("the fleet id is the number shown by fleet list")
	}

	query := url.Values{}

	for _, imei := range positionals[1:] {
		if len(imei) != 15 || strings.ContainsFunc(imei, func(digit rune) bool { return digit < '0' || digit > '9' }) {
			return fmt.Errorf("%s is not an IMEI, which is the 15-digit number printed on the device", api.Printable(imei))
		}

		query.Add("imei", imei)
	}

	if offset > 0 && count == 0 {
		return errors.New("-o sits the window back from the newest logs, so it needs -n")
	}

	// A window of past logs
	if count > 0 {
		query.Set("last", strconv.FormatInt(count, 10))

		if offset > 0 {
			query.Set("offset", strconv.FormatInt(offset, 10))
		}

		answer, err := api.FetchLogs(invocation, fleetID, query)

		if err != nil {
			return err
		}

		for _, entry := range answer.Logs {
			printLog(invocation.Out, entry)
		}

		return nil
	}

	// The newest logs, then follow
	query.Set("last", "100")

	answer, err := api.FetchLogs(invocation, fleetID, query)

	if err != nil {
		return err
	}

	for _, entry := range answer.Logs {
		printLog(invocation.Out, entry)
	}

	query.Del("last")
	cursor := answer.Next

	for {
		query.Set("after", strconv.FormatInt(cursor, 10))

		answer, err = api.FetchLogs(invocation, fleetID, query)

		var unavailable api.Unavailable

		if errors.As(err, &unavailable) {
			time.Sleep(time.Second)
			continue
		}

		if err != nil {
			return err
		}

		for _, entry := range answer.Logs {
			printLog(invocation.Out, entry)
		}

		cursor = answer.Next
	}
}

func printLog(out io.Writer, entry api.LogEntry) {
	name := ""

	if entry.Name != nil {
		name = printableKeepingTabs(*entry.Name)
	}

	fields := fmt.Sprintf("%s %s %s [%s] ",
		entry.ReceivedAt.Local().Format("2006-01-02T15:04:05-07:00"), entry.Imei, entry.Kind, name)
	lines := strings.Split(entry.Text, "\n")

	fmt.Fprintf(out, "%s%s\n", fields, printableKeepingTabs(lines[0]))

	for _, line := range lines[1:] {
		fmt.Fprintf(out, "%s%s\n", strings.Repeat(" ", utf8.RuneCountInString(fields)), printableKeepingTabs(line))
	}
}

func printableKeepingTabs(text string) string {
	segments := strings.Split(text, "\t")

	for index, segment := range segments {
		segments[index] = api.Printable(segment)
	}

	return strings.Join(segments, "\t")
}
