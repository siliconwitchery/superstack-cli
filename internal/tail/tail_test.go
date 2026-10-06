package tail

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/siliconwitchery/superstack-cli/internal/api"
	"github.com/siliconwitchery/superstack-cli/internal/api/apitest"
)

var testReceivedAt = time.Date(2026, 10, 6, 13, 4, 5, 678000000, time.UTC)

// Serves stored logs by the paging rules of GET /fleets/{id}/logs, and records each query it answers
type testLogServer struct {
	mutex   sync.Mutex
	logs    []api.LogEntry
	queries []string
	follow  []api.LogsAnswer
}

func (server *testLogServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	server.mutex.Lock()
	defer server.mutex.Unlock()

	server.queries = append(server.queries, r.URL.RawQuery)

	if r.URL.Path != "/fleets/3/logs" {
		http.Error(w, "no such fleet", http.StatusNotFound)
		return
	}

	query := r.URL.Query()
	selected := []api.LogEntry{}

	for _, entry := range server.logs {
		if !query.Has("imei") || slices.Contains(query["imei"], entry.Imei) {
			selected = append(selected, entry)
		}
	}

	answer := api.LogsAnswer{Logs: []api.LogEntry{}}

	if query.Has("last") {
		last, _ := strconv.ParseInt(query.Get("last"), 10, 64)
		window := selected[len(selected)-int(min(last, int64(len(selected)))):]
		answer.Logs = window[:min(len(window), 1000)]

		if len(selected) > 0 {
			answer.Next = selected[len(selected)-1].Id
		}
	} else {
		if len(server.follow) > 0 {
			answer = server.follow[0]
			server.follow = server.follow[1:]
		} else if server.follow != nil {
			http.Error(w, "the server is restarting", http.StatusServiceUnavailable)
			return
		} else {
			after, _ := strconv.ParseInt(query.Get("after"), 10, 64)
			answer.Next = after

			for _, entry := range selected {
				if entry.Id > after && len(answer.Logs) < 1000 {
					answer.Logs = append(answer.Logs, entry)
				}
			}
		}
	}

	if len(answer.Logs) > 0 {
		answer.Next = answer.Logs[len(answer.Logs)-1].Id
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(answer)
}

func testLogs(count int) []api.LogEntry {
	logs := []api.LogEntry{}
	name := "boiler"

	for id := int64(1); id <= int64(count); id++ {
		imei := "111111111111111"

		if id%2 == 0 {
			imei = "222222222222222"
		}

		logs = append(logs, api.LogEntry{
			Id: id, Imei: imei, Name: &name, Kind: "print", Text: "log " + strconv.FormatInt(id, 10), ReceivedAt: testReceivedAt,
		})
	}

	return logs
}

func printedTexts(output string) []string {
	texts := []string{}

	for _, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		if line == "" {
			continue
		}

		_, text, _ := strings.Cut(line, "] ")
		texts = append(texts, text)
	}

	return texts
}

func TestTailRefusesArguments(t *testing.T) {
	tests := []struct {
		arguments []string
		wantError string
	}{
		{[]string{}, "takes a fleet id"},
		{[]string{"pilot"}, "number shown by fleet list"},
		{[]string{"0"}, "number shown by fleet list"},
		{[]string{"3", "12345"}, "12345 is not an IMEI"},
		{[]string{"3", "-n"}, "-n needs a number"},
		{[]string{"3", "-n", "-1"}, "-n takes a whole number"},
		{[]string{"3", "-n", "ten"}, "-n takes a whole number"},
		{[]string{"3", "--offset", "5"}, "it needs -n"},
		{[]string{"3", "-n", "9223372036854775807", "--offset", "1"}, "more logs than can be counted"},
	}

	for _, test := range tests {
		t.Run(strings.Join(test.arguments, " "), func(t *testing.T) {
			server := &testLogServer{}
			invocation, _ := apitest.LoggedInInvocation(t, server)

			err := Tail(invocation, test.arguments)

			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("error = %v, want it to contain %q", err, test.wantError)
			}

			if len(server.queries) != 0 {
				t.Fatalf("asked the server %v before refusing", server.queries)
			}
		})
	}
}

func TestTailPrintsTheNewestLogs(t *testing.T) {
	tests := []struct {
		name        string
		arguments   []string
		stored      int
		wantTexts   []string
		wantQueries []string
	}{
		{"the newest two", []string{"3", "-n", "2"}, 5, []string{"log 4", "log 5"}, []string{"last=2"}},
		{"more than are stored", []string{"3", "-n", "10"}, 3, []string{"log 1", "log 2", "log 3"}, []string{"last=10"}},
		{"none asked", []string{"3", "-n", "0"}, 5, []string{}, nil},
		{"none stored", []string{"3", "-n", "5"}, 0, []string{}, []string{"last=5"}},
		{"one device", []string{"3", "111111111111111", "-n", "2"}, 5, []string{"log 3", "log 5"}, []string{"imei=111111111111111&last=2"}},
		{"two devices", []string{"3", "111111111111111", "222222222222222", "-n", "1"}, 5, []string{"log 5"}, []string{"imei=111111111111111&imei=222222222222222&last=1"}},
		{"skipping the newest", []string{"3", "-n", "2", "--offset", "1"}, 5, []string{"log 3", "log 4"}, []string{"last=1", "last=3"}},
		{"a page that runs out", []string{"3", "-n", "2", "--offset", "4"}, 5, []string{"log 1"}, []string{"last=4", "last=6"}},
		{"skipping every log", []string{"3", "-n", "2", "--offset", "5"}, 5, []string{}, []string{"last=5", "last=7"}},
		{"skipping more than are stored", []string{"3", "-n", "2", "--offset", "9"}, 5, []string{}, []string{"last=9"}},
		{"flags before the fleet", []string{"-n", "1", "3"}, 5, []string{"log 5"}, []string{"last=1"}},
		{"paging past one answer", []string{"3", "-n", "1200"}, 2500, nil, []string{"last=1200", "after=2300"}},
		{"paging past one answer with an offset", []string{"3", "-n", "1200", "--offset", "100"}, 2500, nil, []string{"last=100", "last=1300", "after=2200"}},
		{"stopping at an answer short of one thousand", []string{"3", "-n", "5000"}, 1500, nil, []string{"last=5000", "after=1000"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := &testLogServer{logs: testLogs(test.stored)}
			invocation, out := apitest.LoggedInInvocation(t, server)

			err := Tail(invocation, test.arguments)

			if err != nil {
				t.Fatal(err)
			}

			texts := printedTexts(out.String())

			if test.wantTexts != nil && !slices.Equal(texts, test.wantTexts) {
				t.Errorf("printed %q, want %q", texts, test.wantTexts)
			}

			if !slices.Equal(server.queries, test.wantQueries) {
				t.Errorf("asked %q, want %q", server.queries, test.wantQueries)
			}
		})
	}

	server := &testLogServer{logs: testLogs(2500)}
	invocation, out := apitest.LoggedInInvocation(t, server)

	err := Tail(invocation, []string{"3", "-n", "1200", "--offset", "100"})

	if err != nil {
		t.Fatal(err)
	}

	texts := printedTexts(out.String())

	if len(texts) != 1200 || texts[0] != "log 1201" || texts[1199] != "log 2400" {
		t.Fatalf("printed %d logs from %q to %q, want 1200 from log 1201 to log 2400",
			len(texts), texts[0], texts[len(texts)-1])
	}
}

func TestTailPrintsEachLineSafely(t *testing.T) {
	name := "roof \x1b[2Junit"
	server := &testLogServer{logs: []api.LogEntry{
		{Id: 1, Imei: "111111111111111", Name: nil, Kind: "state", Text: "Code started", ReceivedAt: testReceivedAt},
		{Id: 2, Imei: "222222222222222", Name: &name, Kind: "print", Text: "21.5\tok\nsecond\x1b]0;title\a‮line", ReceivedAt: testReceivedAt},
	}}
	invocation, out := apitest.LoggedInInvocation(t, server)

	err := Tail(invocation, []string{"3", "-n", "2"})

	if err != nil {
		t.Fatal(err)
	}

	received := testReceivedAt.Local().Format("2006-01-02T15:04:05-07:00")
	want := fmt.Sprintf("%[1]s 111111111111111 state [] Code started\n"+
		"%[1]s 222222222222222 print [roof \\x1b[2Junit] 21.5\tok\n"+
		"%[1]s 222222222222222 print [roof \\x1b[2Junit] second\\x1b]0;title\\a\\u202eline\n", received)

	if out.String() != want {
		t.Fatalf("printed\n%q\nwant\n%q", out.String(), want)
	}

	if strings.Contains(received, " ") || strings.Contains(received, ".") {
		t.Fatalf("received time %q holds a space or fractions of a second", received)
	}
}

func TestTailFollows(t *testing.T) {
	logs := testLogs(7)
	server := &testLogServer{
		logs: logs[:5],
		follow: []api.LogsAnswer{
			{Logs: logs[5:6]},
			{Logs: []api.LogEntry{}, Next: 6},
			{Logs: logs[6:7]},
		},
	}
	invocation, out := apitest.LoggedInInvocation(t, server)

	err := Tail(invocation, []string{"3", "222222222222222"})

	if err == nil || !strings.Contains(err.Error(), "the server is restarting") {
		t.Fatalf("error = %v, want the server's refusal", err)
	}

	texts := printedTexts(out.String())

	if !slices.Equal(texts, []string{"log 6", "log 7"}) {
		t.Errorf("printed %q, want only the logs that arrived while following", texts)
	}

	wantQueries := []string{
		"imei=222222222222222&last=0",
		"after=4&imei=222222222222222",
		"after=6&imei=222222222222222",
		"after=6&imei=222222222222222",
		"after=7&imei=222222222222222",
	}

	if !slices.Equal(server.queries, wantQueries) {
		t.Errorf("asked %q, want %q", server.queries, wantQueries)
	}
}

func TestTailReportsTheServersRefusal(t *testing.T) {
	invocation, _ := apitest.LoggedInInvocation(t, &testLogServer{})

	for _, arguments := range [][]string{{"9"}, {"9", "-n", "1"}} {
		err := Tail(invocation, arguments)

		if err == nil || err.Error() != "no such fleet" {
			t.Errorf("tail %v error = %v, want the server's refusal", arguments, err)
		}
	}
}
