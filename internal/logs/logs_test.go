package logs

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/siliconwitchery/superstack-cli/internal/api"
	"github.com/siliconwitchery/superstack-cli/internal/api/apitest"
)

const kitchen = "111111111111111"
const porch = "222222222222222"

func init() {
	time.Local = time.FixedZone("", 2*60*60)
}

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

// Served in UTC and shown as 2026-01-15T12:01 and that many seconds, at +02:00.
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

// One full answer from the server when from and to are 1,000 apart.
func page(t *testing.T, from int, to int) scriptedAnswer {
	t.Helper()

	logs := []servedLog{}

	for id := from; id <= to; id++ {
		logs = append(logs, served(id, kitchen, "kitchen", "lua", "tick "+strconv.Itoa(id)))
	}

	return answer(t, int64(to), logs...)
}

func printedPage(from int, to int) string {
	lines := strings.Builder{}

	for id := from; id <= to; id++ {
		receivedAt := time.Date(2026, time.January, 15, 12, 1, id, 0, time.Local)

		lines.WriteString(receivedAt.Format("2006-01-02T15:04:05-07:00") + " 111111111111111 lua [kitchen] tick " + strconv.Itoa(id) + "\n")
	}

	return lines.String()
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

func TestTail(t *testing.T) {
	const usage = "tail takes one fleet id, then optional IMEIs"
	const over = "the test is over"

	tests := []struct {
		name        string
		arguments   []string
		answers     []scriptedAnswer
		loggedOut   bool
		wantQueries []string
		wantOutput  string
		wantNotices string
		wantError   string
		wantElapsed time.Duration
	}{
		{
			name:      "earlier logs, then new logs, for a whole fleet",
			arguments: []string{"3"},
			answers: []scriptedAnswer{
				answer(t, 2, served(1, kitchen, "back door", "lua", "hello\t1"), served(2, porch, "", "lifecycle", "Code started")),
				answer(t, 3, served(3, kitchen, "back door", "error", "Code crashed: main.lua:3: attempt to index a nil value")),
				answer(t, 3),
				answer(t, 4, served(4, porch, "", "lua", "tock")),
			},
			wantQueries: []string{"last=10", "after=2", "after=3", "after=3", "after=4"},
			wantOutput: "2026-01-15T12:01:01+02:00 111111111111111 lua [back door] hello\t1\n" +
				"2026-01-15T12:01:02+02:00 222222222222222 lifecycle [] Code started\n" +
				"2026-01-15T12:01:03+02:00 111111111111111 error [back door] Code crashed: main.lua:3: attempt to index a nil value\n" +
				"2026-01-15T12:01:04+02:00 222222222222222 lua [] tock\n",
			wantError: over,
		},
		{
			name:      "several IMEIs",
			arguments: []string{"3", porch, kitchen},
			answers: []scriptedAnswer{
				answer(t, 2, served(1, kitchen, "kitchen", "lua", "hello"), served(2, porch, "", "lua", "ready")),
			},
			wantQueries: []string{
				"imei=222222222222222&imei=111111111111111&last=10",
				"after=2&imei=222222222222222&imei=111111111111111",
			},
			wantOutput: "2026-01-15T12:01:01+02:00 111111111111111 lua [kitchen] hello\n" +
				"2026-01-15T12:01:02+02:00 222222222222222 lua [] ready\n",
			wantError: over,
		},
		{
			name:      "a log of several lines carries every field on each",
			arguments: []string{"3"},
			answers: []scriptedAnswer{
				answer(t, 1, served(1, porch, "", "error", "Code crashed: main.lua:3: boom\nstack traceback:\n\tmain.lua:3: in main chunk")),
			},
			wantQueries: []string{"last=10", "after=1"},
			wantOutput: "2026-01-15T12:01:01+02:00 222222222222222 error [] Code crashed: main.lua:3: boom\n" +
				"2026-01-15T12:01:01+02:00 222222222222222 error [] stack traceback:\n" +
				"2026-01-15T12:01:01+02:00 222222222222222 error [] \tmain.lua:3: in main chunk\n",
			wantError: over,
		},
		{
			name:      "a device name cannot drive the terminal",
			arguments: []string{"3"},
			answers: []scriptedAnswer{
				answer(t, 1, served(1, kitchen, "\x1b[2K\rkitchen", "lua", "hello")),
			},
			wantQueries: []string{"last=10", "after=1"},
			wantOutput:  "2026-01-15T12:01:01+02:00 111111111111111 lua [\\x1b[2K\\rkitchen] hello\n",
			wantError:   over,
		},
		{
			name:        "-n under 1,000, given first, with an IMEI",
			arguments:   []string{"-n", "3", "3", kitchen},
			answers:     []scriptedAnswer{page(t, 5, 7)},
			wantQueries: []string{"imei=111111111111111&last=3"},
			wantOutput:  printedPage(5, 7),
		},
		{
			name:        "-n over 1,000 pages to the newest log",
			arguments:   []string{"3", "-n", "2500"},
			answers:     []scriptedAnswer{page(t, 1, 1000), page(t, 1001, 2000), page(t, 2001, 2500)},
			wantQueries: []string{"last=2500", "after=1000", "after=2000"},
			wantOutput:  printedPage(1, 2500),
		},
		{
			name:        "-n of exactly two full answers asks for no third",
			arguments:   []string{"3", "-n", "2000"},
			answers:     []scriptedAnswer{page(t, 1, 1000), page(t, 1001, 2000)},
			wantQueries: []string{"last=2000", "after=1000"},
			wantOutput:  printedPage(1, 2000),
		},
		{
			name:        "-n prints no more than asked when logs arrive meanwhile",
			arguments:   []string{"3", "-n", "1500"},
			answers:     []scriptedAnswer{page(t, 1, 1000), page(t, 1001, 2000)},
			wantQueries: []string{"last=1500", "after=1000"},
			wantOutput:  printedPage(1, 1500),
		},
		{
			name:        "-n above the logs the server keeps prints them all",
			arguments:   []string{"3", "-n", "5000"},
			answers:     []scriptedAnswer{page(t, 1, 1000), page(t, 1001, 1200)},
			wantQueries: []string{"last=5000", "after=1000"},
			wantOutput:  printedPage(1, 1200),
		},
		{
			name:        "-n 0 prints nothing",
			arguments:   []string{"3", "-n", "0"},
			answers:     []scriptedAnswer{answer(t, 41)},
			wantQueries: []string{"last=0"},
		},
		{
			name:      "a refused, a dropped, and a cut-short answer in a row, then recovery",
			arguments: []string{"3"},
			answers: []scriptedAnswer{
				answer(t, 1, served(1, kitchen, "kitchen", "lua", "before")),
				{status: http.StatusServiceUnavailable, body: "the server could not read the logs"},
				{drop: true},
				{status: http.StatusOK, body: `{"logs":[{"id":2,"imei":"111111111111111","name":"kitchen","kind":"lua","text":"dur`},
				{status: http.StatusBadGateway},
				{status: http.StatusServiceUnavailable, body: "the server could not check the fleet"},
				answer(t, 3, served(2, kitchen, "kitchen", "lua", "during"), served(3, kitchen, "kitchen", "lua", "after")),
			},
			wantQueries: []string{"last=10", "after=1", "after=1", "after=1", "after=1", "after=1", "after=1", "after=3"},
			wantOutput: "2026-01-15T12:01:01+02:00 111111111111111 lua [kitchen] before\n" +
				"2026-01-15T12:01:02+02:00 111111111111111 lua [kitchen] during\n" +
				"2026-01-15T12:01:03+02:00 111111111111111 lua [kitchen] after\n",
			wantNotices: "2000-01-01T02:00:00+02:00 superstack: The server stopped answering. Trying again.\n" +
				"2000-01-01T02:00:27+02:00 superstack: The server is answering again.\n",
			wantError:   over,
			wantElapsed: 27 * time.Second,
		},
		{
			name:      "-n waits for a server that does not answer at first",
			arguments: []string{"3", "-n", "1"},
			answers: []scriptedAnswer{
				{status: http.StatusServiceUnavailable, body: "the server could not read the logs"},
				page(t, 1, 1),
			},
			wantQueries: []string{"last=1", "last=1"},
			wantOutput:  printedPage(1, 1),
			wantNotices: "2000-01-01T02:00:00+02:00 superstack: The server stopped answering. Trying again.\n" +
				"2000-01-01T02:00:01+02:00 superstack: The server is answering again.\n",
			wantElapsed: time.Second,
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
			name:      "a refusal while following ends the command",
			arguments: []string{"3"},
			answers: []scriptedAnswer{
				answer(t, 1, served(1, kitchen, "kitchen", "lua", "hello")),
				{status: http.StatusTooManyRequests, body: "you already have 8 log reads open, close one first"},
			},
			wantQueries: []string{"last=10", "after=1"},
			wantOutput:  "2026-01-15T12:01:01+02:00 111111111111111 lua [kitchen] hello\n",
			wantError:   "you already have 8 log reads open, close one first",
		},
		{name: "no arguments", wantError: usage},
		{name: "two fleet ids", arguments: []string{"3", "4"}, wantError: usage},
		{name: "an IMEI before the fleet id", arguments: []string{kitchen, "3"}, wantError: usage},
		{name: "a wordy fleet id", arguments: []string{"pilot"}, wantError: "the fleet id is the number shown by fleet list"},
		{name: "fleet id zero", arguments: []string{"0", kitchen}, wantError: "the fleet id is the number shown by fleet list"},
		{name: "-n that is not a number", arguments: []string{"3", "-n", "many"}, wantError: "-n needs the number of logs to show"},
		{name: "-n below zero", arguments: []string{"3", "-n", "-1"}, wantError: "-n needs the number of logs to show"},
		{name: "-n with nothing after it", arguments: []string{"3", "-n"}, wantError: "-n needs the number of logs to show"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			invocation, out, seen := serveLogs(t, test.answers)

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

			notices, err := os.CreateTemp(t.TempDir(), "notices")

			if err != nil {
				t.Fatal(err)
			}

			defer notices.Close()

			errorStream := os.Stderr
			os.Stderr = notices

			defer func() { os.Stderr = errorStream }()

			// The retries sleep on the bubble's clock, which starts at midnight UTC on 2000-01-01.
			synctest.Test(t, func(t *testing.T) {
				started := time.Now()

				err := Tail(invocation, test.arguments)

				elapsed := time.Since(started)

				if test.wantError == "" && err != nil {
					t.Errorf("error = %v, want none", err)
				}

				if test.wantError != "" && (err == nil || !strings.Contains(err.Error(), test.wantError)) {
					t.Errorf("error = %v, want %q", err, test.wantError)
				}

				if elapsed != test.wantElapsed {
					t.Errorf("it slept %s between the retries, want %s", elapsed, test.wantElapsed)
				}
			})

			if !slices.Equal(seen(), test.wantQueries) {
				t.Errorf("queries = %q, want %q", seen(), test.wantQueries)
			}

			if out.String() != test.wantOutput {
				t.Errorf("output = %q, want %q", out.String(), test.wantOutput)
			}

			written, err := os.ReadFile(notices.Name())

			if err != nil {
				t.Fatal(err)
			}

			if string(written) != test.wantNotices {
				t.Errorf("the error stream holds %q, want %q", written, test.wantNotices)
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
		{name: "a carriage return before a newline", text: "first\r\nsecond", want: []string{`first\r`, "second"}},
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
				body:   `{"logs":[{"id":1,"imei":"111111111111111","name":"kitchen","kind":"lua","text":` + servedText + `,"received_at":"2026-01-15T10:01:01Z"}],"next":1}`,
			}})

			err := Tail(invocation, []string{"3", "-n", "1"})

			if err != nil {
				t.Fatalf("error = %v", err)
			}

			want := ""

			for _, line := range test.want {
				want += "2026-01-15T12:01:01+02:00 111111111111111 lua [kitchen] " + line + "\n"
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

// Re-runs this test binary as a child that follows the fleet's logs, so the
// interrupt reaches a whole process as Ctrl-C does.
func TestCtrlCEndsTail(t *testing.T) {
	base, isChild := os.LookupEnv("SUPERSTACK_TAIL_SERVER")

	if isChild {
		t.Fatal(Tail(api.NewInvocation(base, "test", os.Stdin, os.Stdout), []string{"3"}))
	}

	if runtime.GOOS == "windows" {
		t.Skip("Windows has no interrupt signal to send to a child")
	}

	invocation, _, _ := serveLogs(t, []scriptedAnswer{
		answer(t, 1, served(1, kitchen, "kitchen", "lua", "hello")),
		{stall: true},
	})

	command := exec.Command(os.Args[0], "-test.run=TestCtrlCEndsTail")
	command.Env = append(os.Environ(), "SUPERSTACK_TAIL_SERVER="+invocation.Base)

	notices := &bytes.Buffer{}
	command.Stderr = notices

	output, err := command.StdoutPipe()

	if err != nil {
		t.Fatal(err)
	}

	err = command.Start()

	if err != nil {
		t.Fatal(err)
	}

	reader := bufio.NewReader(output)

	shown, err := reader.ReadString('\n')

	if err != nil {
		t.Fatalf("tail printed %q and then %v, having said %q", shown, err, notices)
	}

	err = command.Process.Signal(os.Interrupt)

	if err != nil {
		t.Fatal(err)
	}

	rest, err := io.ReadAll(reader)

	if err != nil {
		t.Fatal(err)
	}

	err = command.Wait()

	if err == nil || err.Error() != "signal: interrupt" {
		t.Errorf("tail ended with %v, want it ended by the interrupt", err)
	}

	if shown+string(rest) != "2026-01-15T12:01:01+02:00 111111111111111 lua [kitchen] hello\n" {
		t.Errorf("output = %q, want the one log", shown+string(rest))
	}

	if notices.String() != "" {
		t.Errorf("the error stream holds %q, want nothing", notices)
	}
}

func TestTheClientTimeoutOutlastsTheWait(t *testing.T) {
	invocation := api.NewInvocation("", "test", strings.NewReader(""), &bytes.Buffer{})

	if invocation.Client.Timeout <= 20*time.Second {
		t.Errorf("the client gives up after %s, inside the 20 seconds the server may hold a request", invocation.Client.Timeout)
	}
}
