package tail

import (
	"encoding/json"
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

// Serves stored logs by the rules of GET /fleets/{id}/logs, and records each query it answers. Each after
// request takes the next follow answer, a nil answer being a refusal because the server is unavailable, and
// one past the end is refused as no such fleet
type testLogServer struct {
	mutex   sync.Mutex
	logs    []api.LogEntry
	queries []string
	follow  []*api.LogsAnswer
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
		offset, _ := strconv.ParseInt(query.Get("offset"), 10, 64)

		if last < 1 || last > 1000 || offset < 0 {
			http.Error(w, "the number of logs is 1 to 1000", http.StatusBadRequest)
			return
		}

		end := max(int64(len(selected))-offset, 0)
		answer.Logs = selected[max(end-last, 0):end]

		if len(selected) > 0 {
			answer.Next = selected[len(selected)-1].Id
		}
	} else {
		if len(server.follow) == 0 {
			http.Error(w, "no such fleet", http.StatusNotFound)
			return
		}

		next := server.follow[0]
		server.follow = server.follow[1:]

		if next == nil {
			http.Error(w, "the server is restarting", http.StatusServiceUnavailable)
			return
		}

		answer = *next
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
		{[]string{"3", "-o"}, "-o needs a number"},
		{[]string{"3", "-n", "0"}, "-n takes a number of logs from 1 to 1000"},
		{[]string{"3", "-n", "1001"}, "-n takes a number of logs from 1 to 1000"},
		{[]string{"3", "-n", "ten"}, "-n takes a number of logs from 1 to 1000"},
		{[]string{"3", "-n", "1", "-o", "0"}, "-o takes how many of the newest logs to skip"},
		{[]string{"3", "-n", "1", "-o", "-1"}, "-o takes how many of the newest logs to skip"},
		{[]string{"3", "-o", "5"}, "it needs -n"},
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

func TestTailPrintsAWindow(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
		stored    int
		wantFirst string
		wantLast  string
		wantCount int
		wantQuery string
	}{
		{"the newest two", []string{"3", "-n", "2"}, 5, "log 4", "log 5", 2, "last=2"},
		{"more than are stored", []string{"3", "-n", "10"}, 3, "log 1", "log 3", 3, "last=10"},
		{"none stored", []string{"3", "-n", "5"}, 0, "", "", 0, "last=5"},
		{"one device", []string{"3", "111111111111111", "-n", "2"}, 5, "log 3", "log 5", 2, "imei=111111111111111&last=2"},
		{"two devices", []string{"3", "111111111111111", "222222222222222", "-n", "1"}, 5, "log 5", "log 5", 1, "imei=111111111111111&imei=222222222222222&last=1"},
		{"sitting back from the newest", []string{"3", "-n", "2", "-o", "1"}, 5, "log 3", "log 4", 2, "last=2&offset=1"},
		{"a window that runs out", []string{"3", "-n", "2", "-o", "4"}, 5, "log 1", "log 1", 1, "last=2&offset=4"},
		{"a window past every log", []string{"3", "-n", "2", "-o", "9"}, 5, "", "", 0, "last=2&offset=9"},
		{"flags before the fleet", []string{"-n", "1", "3"}, 5, "log 5", "log 5", 1, "last=1"},
		{"the largest window", []string{"3", "-n", "1000"}, 2500, "log 1501", "log 2500", 1000, "last=1000"},
		{"the largest window sitting back", []string{"3", "-n", "1000", "-o", "100"}, 2500, "log 1401", "log 2400", 1000, "last=1000&offset=100"},
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

			if len(texts) != test.wantCount || (test.wantCount > 0 && (texts[0] != test.wantFirst || texts[len(texts)-1] != test.wantLast)) {
				t.Errorf("printed %d logs %q, want %d from %q to %q", len(texts), texts, test.wantCount, test.wantFirst, test.wantLast)
			}

			if !slices.Equal(server.queries, []string{test.wantQuery}) {
				t.Errorf("asked %q, want only %q", server.queries, test.wantQuery)
			}
		})
	}
}

func TestTailPrintsEachLogSafely(t *testing.T) {
	name := "roof \x1b[2Junit"
	server := &testLogServer{logs: []api.LogEntry{
		{Id: 1, Imei: "111111111111111", Name: nil, Kind: "state", Text: "Code started", ReceivedAt: testReceivedAt},
		{Id: 2, Imei: "222222222222222", Name: &name, Kind: "print", Text: "21.5\tok\nsecond\x1b]0;title\a‮line\n\nsaid \"hi\" in C:\\temp\nlast", ReceivedAt: testReceivedAt},
	}}
	invocation, out := apitest.LoggedInInvocation(t, server)

	err := Tail(invocation, []string{"3", "-n", "2"})

	if err != nil {
		t.Fatal(err)
	}

	received := testReceivedAt.Local().Format("2006-01-02T15:04:05-07:00")
	fields := received + " 222222222222222 print [roof \\x1b[2Junit] "
	indent := strings.Repeat(" ", len(fields))
	want := received + " 111111111111111 state [] Code started\n" +
		fields + "21.5\tok\n" +
		indent + "second\\x1b]0;title\\a\\u202eline\n" +
		indent + "\n" +
		indent + "said \"hi\" in C:\\temp\n" +
		indent + "last\n"

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
		follow: []*api.LogsAnswer{
			{Logs: logs[5:6]},
			nil,
			{Logs: []api.LogEntry{}, Next: 6},
			{Logs: logs[6:7]},
		},
	}
	invocation, out := apitest.LoggedInInvocation(t, server)

	err := Tail(invocation, []string{"3", "222222222222222"})

	if err == nil || err.Error() != "no such fleet" {
		t.Fatalf("error = %v, want the server's refusal that ends the follow", err)
	}

	texts := printedTexts(out.String())

	if !slices.Equal(texts, []string{"log 2", "log 4", "log 6", "log 7"}) {
		t.Errorf("printed %q, want the newest logs then the ones that arrived while following", texts)
	}

	wantQueries := []string{
		"imei=222222222222222&last=100",
		"after=4&imei=222222222222222",
		"after=6&imei=222222222222222",
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
