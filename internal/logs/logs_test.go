package logs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/siliconwitchery/superstack-cli/internal/api"
	"github.com/siliconwitchery/superstack-cli/internal/api/apitest"
)

const kitchen = "111111111111111"
const porch = "222222222222222"

type scriptedAnswer struct {
	status int
	body   string
	drop   bool
	stall  bool
}

type servedLog struct {
	Id         int64   `json:"id"`
	Imei       string  `json:"imei"`
	Name       *string `json:"name"`
	Kind       string  `json:"kind"`
	Text       string  `json:"text"`
	ReceivedAt string  `json:"received_at"`
}

// Served in UTC and shown as 2026-01-15 12:01 and that many seconds, whatever zone the test runs in.
func served(second int, imei string, name string, kind string, text string) servedLog {
	log := servedLog{
		Id:         int64(second),
		Imei:       imei,
		Kind:       kind,
		Text:       text,
		ReceivedAt: time.Date(2026, time.January, 15, 12, 1, second, 0, time.Local).UTC().Format(time.RFC3339),
	}

	if name != "" {
		log.Name = &name
	}

	return log
}

func answer(t *testing.T, next int64, logs ...servedLog) scriptedAnswer {
	t.Helper()

	if logs == nil {
		logs = []servedLog{}
	}

	body, err := json.Marshal(struct {
		Logs []servedLog `json:"logs"`
		Next int64       `json:"next"`
	}{logs, next})

	if err != nil {
		t.Fatal(err)
	}

	return scriptedAnswer{status: http.StatusOK, body: string(body)}
}

// Serves the answers in order, then refuses with "the test is over", which
// ends a tail that would otherwise run until Ctrl-C.
func serveLogs(t *testing.T, answers []scriptedAnswer) (api.Invocation, *bytes.Buffer, func() []string) {
	t.Helper()

	lock := sync.Mutex{}
	queries := []string{}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /fleets/3/logs", func(w http.ResponseWriter, r *http.Request) {
		lock.Lock()
		index := len(queries)
		queries = append(queries, r.URL.RawQuery)
		lock.Unlock()

		if index >= len(answers) {
			http.Error(w, "the test is over", http.StatusGone)
			return
		}

		switch {
		case answers[index].drop:
			connection, _, err := w.(http.Hijacker).Hijack()

			if err != nil {
				t.Error(err)
				return
			}

			connection.Close()

		case answers[index].stall:
			select {
			case <-r.Context().Done():
			case <-time.After(10 * time.Second):
			}

		default:
			w.WriteHeader(answers[index].status)
			fmt.Fprint(w, answers[index].body)
		}
	})

	invocation, out := apitest.LoggedInInvocation(t, mux)

	// A reused connection would let the transport quietly resend a dropped request.
	client := *invocation.Client
	client.Transport = &http.Transport{DisableKeepAlives: true}
	invocation.Client = &client

	seen := func() []string {
		lock.Lock()
		defer lock.Unlock()

		return slices.Clone(queries)
	}

	return invocation, out, seen
}

// Checks that each notice leads with the local time it was printed, then
// swaps that time for <now> so the rest of the output compares exactly.
func checkNoticeTimes(t *testing.T, shown string, started time.Time) string {
	t.Helper()

	lines := strings.SplitAfter(shown, "\n")

	for index, line := range lines {
		if len(line) < 19 || !strings.HasPrefix(line[19:], " superstack: ") {
			continue
		}

		printedAt, err := time.ParseInLocation("2006-01-02 15:04:05", line[:19], time.Local)

		if err != nil || printedAt.Before(started.Truncate(time.Second)) || printedAt.After(time.Now()) {
			t.Errorf("the notice %q does not lead with the local time it was printed", line)
		}

		lines[index] = "<now>" + line[19:]
	}

	return strings.Join(lines, "")
}

func TestTail(t *testing.T) {
	const usage = "tail takes one fleet id, then optional IMEIs"

	tests := []struct {
		name          string
		arguments     []string
		answers       []scriptedAnswer
		loggedOut     bool
		clientTimeout time.Duration
		wantQueries   []string
		wantOutput    string
		wantError     string
		wantAtLeast   time.Duration
	}{
		{
			name:      "history, then new logs, for a whole fleet",
			arguments: []string{"3"},
			answers: []scriptedAnswer{
				answer(t, 2, served(1, kitchen, "kitchen", "lua", "hello\t1"), served(2, porch, "", "lua", "ready")),
				answer(t, 3, served(3, kitchen, "kitchen", "lua", "tick")),
				answer(t, 3),
				answer(t, 5, served(4, porch, "", "lua", "tock"), served(5, kitchen, "kitchen", "lua", "tick")),
			},
			wantQueries: []string{"last=10", "after=2&wait=20", "after=3&wait=20", "after=3&wait=20", "after=5&wait=20"},
			wantOutput: "2026-01-15 12:01:01 kitchen[lua]: hello\t1\n" +
				"2026-01-15 12:01:02 222222222222222[lua]: ready\n" +
				"2026-01-15 12:01:03 kitchen[lua]: tick\n" +
				"2026-01-15 12:01:04 222222222222222[lua]: tock\n" +
				"2026-01-15 12:01:05 kitchen[lua]: tick\n",
		},
		{
			name:      "one IMEI",
			arguments: []string{"3", kitchen},
			answers: []scriptedAnswer{
				answer(t, 1, served(1, kitchen, "kitchen", "lua", "hello")),
				answer(t, 3, served(3, kitchen, "kitchen", "lua", "tick")),
			},
			wantQueries: []string{
				"imei=111111111111111&last=10",
				"after=1&imei=111111111111111&wait=20",
				"after=3&imei=111111111111111&wait=20",
			},
			wantOutput: "2026-01-15 12:01:01 kitchen[lua]: hello\n2026-01-15 12:01:03 kitchen[lua]: tick\n",
		},
		{
			name:      "several IMEIs",
			arguments: []string{"3", porch, kitchen},
			answers: []scriptedAnswer{
				answer(t, 2, served(1, kitchen, "kitchen", "lua", "hello"), served(2, porch, "", "lua", "ready")),
			},
			wantQueries: []string{
				"imei=222222222222222&imei=111111111111111&last=10",
				"after=2&imei=222222222222222&imei=111111111111111&wait=20",
			},
			wantOutput: "2026-01-15 12:01:01 kitchen[lua]: hello\n2026-01-15 12:01:02 222222222222222[lua]: ready\n",
		},
		{
			name:      "a fleet with no logs yet",
			arguments: []string{"3"},
			answers: []scriptedAnswer{
				answer(t, 0),
				answer(t, 1, served(1, kitchen, "kitchen", "lifecycle", "Code started")),
			},
			wantQueries: []string{"last=10", "after=0&wait=20", "after=1&wait=20"},
			wantOutput:  "2026-01-15 12:01:01 kitchen[lifecycle]: Code started\n",
		},
		{
			name:      "a device stops, takes new code, and starts again",
			arguments: []string{"3"},
			answers: []scriptedAnswer{
				answer(t, 1, served(1, kitchen, "kitchen", "lua", "old code 1")),
				answer(t, 2, served(2, kitchen, "kitchen", "lifecycle", "Code stopped")),
				answer(t, 4, served(3, kitchen, "kitchen", "lifecycle", "Code started"), served(4, kitchen, "kitchen", "lua", "new code\t1")),
				answer(t, 5, served(5, kitchen, "kitchen", "error", "Code crashed: main.lua:3: attempt to index a nil value")),
			},
			wantQueries: []string{"last=10", "after=1&wait=20", "after=2&wait=20", "after=4&wait=20", "after=5&wait=20"},
			wantOutput: "2026-01-15 12:01:01 kitchen[lua]: old code 1\n" +
				"2026-01-15 12:01:02 kitchen[lifecycle]: Code stopped\n" +
				"2026-01-15 12:01:03 kitchen[lifecycle]: Code started\n" +
				"2026-01-15 12:01:04 kitchen[lua]: new code\t1\n" +
				"2026-01-15 12:01:05 kitchen[error]: Code crashed: main.lua:3: attempt to index a nil value\n",
		},
		{
			name:      "an error that spans lines names its device on each",
			arguments: []string{"3"},
			answers: []scriptedAnswer{
				answer(t, 1, served(1, porch, "", "error", "Code crashed: main.lua:3: boom\nstack traceback:\n\tmain.lua:3: in main chunk")),
			},
			wantQueries: []string{"last=10", "after=1&wait=20"},
			wantOutput: "2026-01-15 12:01:01 222222222222222[error]: Code crashed: main.lua:3: boom\n" +
				"2026-01-15 12:01:01 222222222222222[error]: stack traceback:\n" +
				"2026-01-15 12:01:01 222222222222222[error]: \tmain.lua:3: in main chunk\n",
		},
		{
			name:      "a device name cannot drive the terminal",
			arguments: []string{"3"},
			answers: []scriptedAnswer{
				answer(t, 1, served(1, kitchen, "\x1b[2K\rkitchen", "lua", "hello")),
			},
			wantQueries: []string{"last=10", "after=1&wait=20"},
			wantOutput:  "2026-01-15 12:01:01 \\x1b[2K\\rkitchen[lua]: hello\n",
		},
		{
			name:      "a device name with spaces, and kinds this CLI does not know",
			arguments: []string{"3"},
			answers: []scriptedAnswer{
				answer(t, 2, served(1, kitchen, "back door", "firmware", "Firmware updated"), served(2, kitchen, "back door", "\x1b[2Jlua", "hello")),
			},
			wantQueries: []string{"last=10", "after=2&wait=20"},
			wantOutput: "2026-01-15 12:01:01 back door[firmware]: Firmware updated\n" +
				"2026-01-15 12:01:02 back door[\\x1b[2Jlua]: hello\n",
		},
		{
			name:      "an unreadable time leaves the rest of the line",
			arguments: []string{"3"},
			answers: []scriptedAnswer{
				{status: http.StatusOK, body: `{"logs":[{"id":1,"imei":"111111111111111","name":"kitchen","kind":"lua","text":"hello","received_at":"yesterday"}],"next":1}`},
			},
			wantQueries: []string{"last=10", "after=1&wait=20"},
			wantOutput:  "---------- --:--:-- kitchen[lua]: hello\n",
		},
		{
			name:      "-n given",
			arguments: []string{"3", "-n", "3"},
			answers: []scriptedAnswer{
				answer(t, 7, served(5, kitchen, "kitchen", "lua", "a"), served(6, kitchen, "kitchen", "lua", "b"), served(7, kitchen, "kitchen", "lua", "c")),
			},
			wantQueries: []string{"last=3", "after=7&wait=20"},
			wantOutput:  "2026-01-15 12:01:05 kitchen[lua]: a\n2026-01-15 12:01:06 kitchen[lua]: b\n2026-01-15 12:01:07 kitchen[lua]: c\n",
		},
		{
			name:        "-n before the fleet id, with an IMEI",
			arguments:   []string{"-n", "1000", "3", kitchen},
			answers:     []scriptedAnswer{answer(t, 0)},
			wantQueries: []string{"imei=111111111111111&last=1000", "after=0&imei=111111111111111&wait=20"},
		},
		{
			name:      "-n 0 shows only new logs",
			arguments: []string{"3", "-n", "0"},
			answers: []scriptedAnswer{
				answer(t, 41),
				answer(t, 42, served(42, kitchen, "kitchen", "lua", "new")),
			},
			wantQueries: []string{"last=0", "after=41&wait=20", "after=42&wait=20"},
			wantOutput:  "2026-01-15 12:01:42 kitchen[lua]: new\n",
		},
		{
			name:      "a refused and a dropped request in a row, then recovery",
			arguments: []string{"3"},
			answers: []scriptedAnswer{
				answer(t, 1, served(1, kitchen, "kitchen", "lua", "before")),
				{status: http.StatusServiceUnavailable, body: "the server is restarting"},
				{drop: true},
				answer(t, 3, served(2, kitchen, "kitchen", "lua", "during"), served(3, kitchen, "kitchen", "lua", "after")),
			},
			wantQueries: []string{"last=10", "after=1&wait=20", "after=1&wait=20", "after=1&wait=20", "after=3&wait=20"},
			wantOutput: "2026-01-15 12:01:01 kitchen[lua]: before\n" +
				"<now> superstack: The server stopped answering. Trying again.\n" +
				"<now> superstack: The server is answering again.\n" +
				"2026-01-15 12:01:02 kitchen[lua]: during\n" +
				"2026-01-15 12:01:03 kitchen[lua]: after\n",
			wantAtLeast: 3 * time.Second,
		},
		{
			name:          "the first request times out, and a later answer is cut short",
			arguments:     []string{"3", "-n", "1"},
			clientTimeout: 200 * time.Millisecond,
			answers: []scriptedAnswer{
				{stall: true},
				answer(t, 1, served(1, kitchen, "kitchen", "lua", "before")),
				{status: http.StatusOK, body: `{"logs":[{"id":2,"imei":"111111111111111","name":"kitchen","kind":"lua","text":"aft`},
				answer(t, 2, served(2, kitchen, "kitchen", "lua", "after")),
			},
			wantQueries: []string{"last=1", "last=1", "after=1&wait=20", "after=1&wait=20", "after=2&wait=20"},
			wantOutput: "<now> superstack: The server stopped answering. Trying again.\n" +
				"<now> superstack: The server is answering again.\n" +
				"2026-01-15 12:01:01 kitchen[lua]: before\n" +
				"<now> superstack: The server stopped answering. Trying again.\n" +
				"<now> superstack: The server is answering again.\n" +
				"2026-01-15 12:01:02 kitchen[lua]: after\n",
			wantAtLeast: 2 * time.Second,
		},
		{
			name:        "the login is no longer valid",
			arguments:   []string{"3"},
			answers:     []scriptedAnswer{{status: http.StatusUnauthorized, body: "the login is no longer valid, log in again"}},
			wantQueries: []string{"last=10"},
			wantError:   "the login is no longer valid, log in again",
		},
		{
			name:      "not logged in",
			arguments: []string{"3"},
			loggedOut: true,
			wantError: "you are not logged in, run login first",
		},
		{
			name:        "no such fleet",
			arguments:   []string{"3"},
			answers:     []scriptedAnswer{{status: http.StatusNotFound, body: "no such fleet"}},
			wantQueries: []string{"last=10"},
			wantError:   "no such fleet",
		},
		{
			name:        "a device outside the fleet",
			arguments:   []string{"3", porch},
			answers:     []scriptedAnswer{{status: http.StatusNotFound, body: "device 222222222222222 is not in that fleet"}},
			wantQueries: []string{"imei=222222222222222&last=10"},
			wantError:   "device 222222222222222 is not in that fleet",
		},
		{
			name:        "an out-of-date CLI",
			arguments:   []string{"3"},
			answers:     []scriptedAnswer{{status: http.StatusUpgradeRequired, body: "update superstack to carry on"}},
			wantQueries: []string{"last=10"},
			wantError:   "update superstack to carry on",
		},
		{
			name:      "a refusal while following ends the command",
			arguments: []string{"3"},
			answers: []scriptedAnswer{
				answer(t, 1, served(1, kitchen, "kitchen", "lua", "hello")),
				{status: http.StatusTooManyRequests, body: "too many tails are open, close one first"},
			},
			wantQueries: []string{"last=10", "after=1&wait=20"},
			wantOutput:  "2026-01-15 12:01:01 kitchen[lua]: hello\n",
			wantError:   "too many tails are open, close one first",
		},
		{name: "no arguments", wantError: usage},
		{name: "two fleet ids", arguments: []string{"3", "4"}, wantError: usage},
		{name: "an IMEI in first place", arguments: []string{kitchen}, wantError: usage},
		{name: "an IMEI before the fleet id", arguments: []string{kitchen, "3"}, wantError: usage},
		{name: "an IMEI that is too short", arguments: []string{"3", "11111111111111"}, wantError: usage},
		{name: "an IMEI with a letter", arguments: []string{"3", kitchen, "22222222222222a"}, wantError: usage},
		{name: "a wordy fleet id", arguments: []string{"pilot"}, wantError: "the fleet id is the number shown by fleet list"},
		{name: "fleet id zero", arguments: []string{"0", kitchen}, wantError: "the fleet id is the number shown by fleet list"},
		{name: "-n that is not a number", arguments: []string{"3", "-n", "many"}, wantError: "-n needs the number of earlier logs to show, from 0 to 1000"},
		{name: "-n below zero", arguments: []string{"3", "-n", "-1"}, wantError: "-n needs the number of earlier logs to show, from 0 to 1000"},
		{name: "-n above the most the server returns", arguments: []string{"3", "-n", "1001"}, wantError: "-n needs the number of earlier logs to show, from 0 to 1000"},
		{name: "-n with nothing after it", arguments: []string{"3", "-n"}, wantError: "-n needs the number of earlier logs to show, from 0 to 1000"},
		{name: "--log-file with nothing after it", arguments: []string{"3", "--log-file"}, wantError: "--log-file needs a file"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			invocation, out, seen := serveLogs(t, test.answers)

			if test.clientTimeout != 0 {
				invocation.Client.Timeout = test.clientTimeout
			}

			if test.loggedOut {
				path, err := api.LoginKeyPath()

				if err != nil {
					t.Fatal(err)
				}

				err = os.Remove(path)

				if err != nil {
					t.Fatal(err)
				}
			}

			started := time.Now()

			err := Tail(invocation, test.arguments)

			elapsed := time.Since(started)

			wantError := test.wantError

			if wantError == "" {
				wantError = "the test is over"
			}

			if err == nil || !strings.Contains(err.Error(), wantError) {
				t.Errorf("error = %v, want %q", err, wantError)
			}

			if !slices.Equal(seen(), test.wantQueries) {
				t.Errorf("queries = %q, want %q", seen(), test.wantQueries)
			}

			shown := checkNoticeTimes(t, out.String(), started)

			if shown != test.wantOutput {
				t.Errorf("output = %q, want %q", shown, test.wantOutput)
			}

			if elapsed < test.wantAtLeast {
				t.Errorf("it took %s, want at least %s between the retries", elapsed, test.wantAtLeast)
			}
		})
	}
}

func TestTailShowsTextAsLuaPrintsIt(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		servedText string
		want       []string
	}{
		{name: "plain text", text: "hello", want: []string{"hello"}},
		{name: "the tab between print's arguments", text: "a\t1\tnil", want: []string{"a\t1\tnil"}},
		{name: "quotes", text: `say "hi" and 'bye'`, want: []string{`say "hi" and 'bye'`}},
		{name: "a backslash", text: `C:\temp\new`, want: []string{`C:\temp\new`}},
		{name: "an embedded newline", text: "first\nsecond", want: []string{"first", "second"}},
		{name: "a trailing newline", text: "first\n", want: []string{"first", ""}},
		{name: "an empty print", text: "", want: []string{""}},
		{name: "other scripts and symbols", text: "気温 21°C \U0001F321", want: []string{"気温 21°C \U0001F321"}},
		{name: "a clear-screen sequence", text: "\x1b[2J\x1b[Hcleared", want: []string{`\x1b[2J\x1b[Hcleared`}},
		{name: "a window title sequence", text: "\x1b]0;title\a", want: []string{`\x1b]0;title\a`}},
		{name: "a carriage return", text: "progress\rdone", want: []string{`progress\rdone`}},
		{name: "a carriage return before a newline", text: "first\r\nsecond", want: []string{`first\r`, "second"}},
		{name: "a bell and a backspace", text: "a\bb\a", want: []string{`a\bb\a`}},
		{name: "a vertical tab and a form feed", text: "a\vb\fc", want: []string{`a\vb\fc`}},
		{name: "a null and a delete", text: "a\x00b\x7fc", want: []string{`a\x00b\x7fc`}},
		{name: "an eight-bit control sequence introducer", text: "a\u009b2Jb", want: []string{`a\u009b2Jb`}},
		{name: "a right-to-left override", text: "a\u202eb", want: []string{`a\u202eb`}},
		{name: "a zero-width joiner", text: "a\u200db", want: []string{`a\u200db`}},
		{name: "bytes that are not UTF-8", servedText: "\"a\xff\xfeb\"", want: []string{"a\ufffd\ufffdb"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			servedText := test.servedText

			if servedText == "" {
				encoded, err := json.Marshal(test.text)

				if err != nil {
					t.Fatal(err)
				}

				servedText = string(encoded)
			}

			invocation, out, _ := serveLogs(t, []scriptedAnswer{{
				status: http.StatusOK,
				body:   `{"logs":[{"id":1,"imei":"111111111111111","name":"kitchen","kind":"lua","text":` + servedText + `,"received_at":"yesterday"}],"next":1}`,
			}})

			err := Tail(invocation, []string{"3"})

			if err == nil || err.Error() != "the test is over" {
				t.Fatalf("error = %v", err)
			}

			want := ""

			for _, line := range test.want {
				want += "---------- --:--:-- kitchen[lua]: " + line + "\n"
			}

			if out.String() != want {
				t.Errorf("output = %q, want %q", out.String(), want)
			}

			for _, letter := range out.String() {
				if letter != '\t' && letter != '\n' && !strconv.IsGraphic(letter) {
					t.Errorf("output %q still holds %U, which a terminal would act on", out.String(), letter)
				}
			}
		})
	}
}

func TestTailLogFile(t *testing.T) {
	tests := []struct {
		name      string
		directory string
		existing  string
		wantError string
	}{
		{name: "a new file receives what the terminal shows"},
		{name: "an existing file is added to", existing: "2026-01-15 12:00:00 kitchen[lua]: earlier\n"},
		{name: "a file that cannot be opened fails before any request", directory: "missing", wantError: "tail.log could not be opened for writing"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			invocation, out, seen := serveLogs(t, []scriptedAnswer{
				answer(t, 1, served(1, kitchen, "kitchen", "lifecycle", "Code started")),
				{status: http.StatusBadGateway, body: "the server is restarting"},
				answer(t, 2, served(2, kitchen, "kitchen", "lua", "first\nsecond")),
			})

			path := filepath.Join(t.TempDir(), test.directory, "tail.log")

			if test.existing != "" {
				err := os.WriteFile(path, []byte(test.existing), 0o644)

				if err != nil {
					t.Fatal(err)
				}
			}

			started := time.Now()

			err := Tail(invocation, []string{"3", "--log-file", path})

			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Errorf("error = %v, want %q", err, test.wantError)
				}

				if len(seen()) != 0 || out.String() != "" {
					t.Errorf("it asked for %q and printed %q before failing", seen(), out.String())
				}

				return
			}

			if err == nil || err.Error() != "the test is over" {
				t.Fatalf("error = %v", err)
			}

			wantOutput := "2026-01-15 12:01:01 kitchen[lifecycle]: Code started\n" +
				"<now> superstack: The server stopped answering. Trying again.\n" +
				"<now> superstack: The server is answering again.\n" +
				"2026-01-15 12:01:02 kitchen[lua]: first\n" +
				"2026-01-15 12:01:02 kitchen[lua]: second\n"

			shown := checkNoticeTimes(t, out.String(), started)

			if shown != wantOutput {
				t.Errorf("output = %q, want %q", shown, wantOutput)
			}

			written, err := os.ReadFile(path)

			if err != nil {
				t.Fatal(err)
			}

			if string(written) != test.existing+out.String() {
				t.Errorf("the log file holds %q, want %q", written, test.existing+out.String())
			}
		})
	}
}

func TestTheClientTimeoutOutlastsTheWait(t *testing.T) {
	invocation := api.NewInvocation("", "test", strings.NewReader(""), &bytes.Buffer{})

	if invocation.Client.Timeout <= 20*time.Second {
		t.Errorf("the client gives up after %s, inside the 20 seconds the server may hold a request", invocation.Client.Timeout)
	}
}
