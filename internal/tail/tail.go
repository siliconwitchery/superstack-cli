package tail

import (
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/siliconwitchery/superstack-cli/internal/api"
)

func Tail(invocation api.Invocation, arguments []string) error {
	positionals := []string{}
	count := int64(-1)
	offset := int64(0)
	offsetGiven := false

	for index := 0; index < len(arguments); index++ {
		switch arguments[index] {
		case "-n", "--offset":
			flag := arguments[index]

			if index+1 == len(arguments) {
				return fmt.Errorf("%s needs a number", flag)
			}

			index++

			value, err := strconv.ParseInt(arguments[index], 10, 64)

			if err != nil || value < 0 {
				return fmt.Errorf("%s takes a whole number of logs", flag)
			}

			if flag == "-n" {
				count = value
			} else {
				offset = value
				offsetGiven = true
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

	imeis := positionals[1:]

	for _, imei := range imeis {
		if len(imei) != 15 || strings.ContainsFunc(imei, func(digit rune) bool { return digit < '0' || digit > '9' }) {
			return fmt.Errorf("%s is not an IMEI, which is the 15-digit number printed on the device", api.Printable(imei))
		}
	}

	if offsetGiven && count < 0 {
		return errors.New("--offset skips the newest logs before -n prints, so it needs -n")
	}

	if count > math.MaxInt64-offset {
		return errors.New("-n and --offset together ask for more logs than can be counted")
	}

	// Follow
	if count < 0 {
		answer, err := api.FetchLogs(invocation, fleetID, imeis, "last", 0)

		if err != nil {
			return err
		}

		for {
			answer, err = api.FetchLogs(invocation, fleetID, imeis, "after", answer.Next)

			if err != nil {
				return err
			}

			for _, entry := range answer.Logs {
				printLog(invocation.Out, entry)
			}
		}
	}

	if count == 0 {
		return nil
	}

	// The oldest of the newest offset logs is the first one not to print
	boundary := int64(math.MaxInt64)

	if offset > 0 {
		answer, err := api.FetchLogs(invocation, fleetID, imeis, "last", offset)

		if err != nil {
			return err
		}

		if int64(len(answer.Logs)) < min(offset, 1000) {
			return nil
		}

		boundary = answer.Logs[0].Id
	}

	answer, err := api.FetchLogs(invocation, fleetID, imeis, "last", count+offset)

	if err != nil {
		return err
	}

	printed := int64(0)

	for {
		for _, entry := range answer.Logs {
			if printed == count || entry.Id >= boundary {
				return nil
			}

			printLog(invocation.Out, entry)
			printed++
		}

		if printed == count || len(answer.Logs) < 1000 {
			return nil
		}

		answer, err = api.FetchLogs(invocation, fleetID, imeis, "after", answer.Next)

		if err != nil {
			return err
		}
	}
}

func printLog(out io.Writer, entry api.LogEntry) {
	received := entry.ReceivedAt.Local().Format("2006-01-02T15:04:05-07:00")
	name := ""

	if entry.Name != nil {
		name = printableKeepingTabs(*entry.Name)
	}

	for _, line := range strings.Split(entry.Text, "\n") {
		fmt.Fprintf(out, "%s %s %s [%s] %s\n", received, entry.Imei, entry.Kind, name, printableKeepingTabs(line))
	}
}

func printableKeepingTabs(text string) string {
	segments := strings.Split(text, "\t")

	for index, segment := range segments {
		segments[index] = api.Printable(segment)
	}

	return strings.Join(segments, "\t")
}
