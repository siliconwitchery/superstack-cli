package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/siliconwitchery/superstack-cli/internal/commands"
)

const version = "0.0.3"

type command struct {
	name      string
	arguments string
	summary   string
	run       func(arguments []string) error
}

type section struct {
	title    string
	commands []command
}

var sections = []section{
	{
		title: "Getting started",
		commands: []command{
			{name: "login", arguments: "<github|gitlab>", summary: "Log in with the selected provider", run: commands.Login},
			{name: "logout", summary: "Log out of your account", run: commands.Logout},
		},
	},
	{
		title: "Fleets",
		commands: []command{
			{name: "fleet create", arguments: "<name>", summary: "Create a fleet", run: commands.FleetCreate},
			{name: "fleet list", summary: "List the fleets you can reach", run: commands.FleetList},
			{name: "fleet rename", arguments: "<fleet_id> <new_name>", summary: "Rename a fleet", run: commands.FleetRename},
			{name: "fleet transfer", arguments: "<fleet_id> <email>", summary: "Hand a fleet to a new owner", run: commands.FleetTransfer},
			{name: "fleet delete", arguments: "<fleet_id>", summary: "Delete a fleet and release its devices", run: commands.FleetDelete},
		},
	},
	{
		title: "Devices",
		commands: []command{
			{name: "device claim", arguments: "<imei> <fleet_id> [name]", summary: "Claim a device into a fleet, then press its button"},
			{name: "device list", arguments: "[fleet_id]", summary: "List devices, their state, and when they were last seen"},
			{name: "device rename", arguments: "<imei> <new_name>", summary: "Rename a device"},
			{name: "device release", arguments: "<imei>", summary: "Unpair a device from its fleet and factory reset it"},
			{name: "device start", arguments: "<imei>", summary: "Run the code on the target"},
			{name: "device stop", arguments: "<imei>", summary: "Halt the code on the target"},
			{name: "device restart", arguments: "<imei>", summary: "Restart the code on the target"},
		},
	},
	{
		title: "Files",
		commands: []command{
			{name: "upload", arguments: "<imei|fleet_id> <file> ...", summary: "Upload files or directories to the target"},
			{name: "download", arguments: "<imei|fleet_id> <path>", summary: "Download the target's files into <path>"},
			{name: "dev", arguments: "<imei|fleet_id> <file> ... [--log-file <file>]", summary: "Upload on every change, and tail"},
		},
	},
	{
		title: "Data",
		commands: []command{
			{name: "tail", arguments: "<imei|fleet_id> [-n num] [--log-file <file>]", summary: "Stream the target's log as it arrives"},
			{name: "send", arguments: "<imei|fleet_id> -m <message>", summary: "Queue a message for the target to collect"},
		},
	},
	{
		title: "People",
		commands: []command{
			{name: "member add", arguments: "<email> <fleet_id>", summary: "Give someone access to a fleet", run: commands.MemberAdd},
			{name: "member list", arguments: "<fleet_id>", summary: "List the people who can reach a fleet", run: commands.MemberList},
			{name: "member remove", arguments: "<email> <fleet_id>", summary: "Take away someone's access", run: commands.MemberRemove},
		},
	},
	{
		title: "Keys",
		commands: []command{
			{name: "key create", arguments: "<fleet_id> [label]", summary: "Create a key for sending data to a fleet"},
			{name: "key list", arguments: "[fleet_id]", summary: "List the keys that can reach your fleets"},
			{name: "key revoke", arguments: "<key_id>", summary: "Stop a key from reaching its fleet"},
		},
	},
	{
		title: "Account",
		commands: []command{
			{name: "account balance", arguments: "[fleet_id]", summary: "Show the credit left on your fleets"},
			{name: "account topup", arguments: "<fleet_id>", summary: "Add credit to a fleet"},
			{name: "account delete", summary: "Delete your account entirely"},
		},
	},
	{
		title: "Superstack",
		commands: []command{
			{name: "version", summary: "Show the version"},
			{name: "help", arguments: "[command]", summary: "Show this help, or help for one command"},
		},
	},
}

func resolve(arguments []string) (command, []string, bool) {
	longest := command{}
	longestWords := 0

	for _, section := range sections {
		for _, candidate := range section.commands {
			words := strings.Fields(candidate.name)

			if len(words) > len(arguments) || len(words) <= longestWords {
				continue
			}

			matches := true

			for index, word := range words {
				if arguments[index] != word {
					matches = false
					break
				}
			}

			if !matches {
				continue
			}

			longest = candidate
			longestWords = len(words)
		}
	}

	if longestWords == 0 {
		return command{}, nil, false
	}

	return longest, arguments[longestWords:], true
}

func printHelp(writer io.Writer) {
	widest := 0

	for _, section := range sections {
		for _, entry := range section.commands {
			width := len(entry.name)

			if entry.arguments != "" {
				width += 1 + len(entry.arguments)
			}

			if width > widest {
				widest = width
			}
		}
	}

	fmt.Fprintf(writer, "superstack %s\n\n", version)
	fmt.Fprint(writer, "Usage: superstack <command> [arguments] [flags]\n")

	for _, section := range sections {
		fmt.Fprintf(writer, "\n%s\n", section.title)

		for _, entry := range section.commands {
			signature := entry.name

			if entry.arguments != "" {
				signature += " " + entry.arguments
			}

			fmt.Fprintf(writer, "  %-*s  %s\n", widest, signature, entry.summary)
		}
	}

	fmt.Fprint(writer, `
Flags
  --json  Print machine-readable output
`)
}

func main() {
	commands.CliVersion = version

	arguments, err := commands.TakeServerFlag(os.Args[1:])

	if err != nil {
		fmt.Fprintf(os.Stderr, "superstack: %s\n", err)
		os.Exit(1)
	}

	if len(arguments) == 0 {
		printHelp(os.Stdout)
		return
	}

	switch arguments[0] {
	case "-h", "--help":
		printHelp(os.Stdout)
		return

	case "-v", "--version":
		fmt.Println(version)
		return
	}

	entry, rest, found := resolve(arguments)

	if !found {
		fmt.Fprintf(os.Stderr, "superstack: unknown command %q\nRun 'superstack help' for the list.\n", strings.Join(arguments, " "))
		os.Exit(1)
	}

	switch entry.name {
	case "version":
		fmt.Println(version)
		return

	case "help":
		if len(rest) == 0 {
			printHelp(os.Stdout)
			return
		}

		topic, _, topicFound := resolve(rest)

		if !topicFound {
			fmt.Fprintf(os.Stderr, "superstack: unknown command %q\n", strings.Join(rest, " "))
			os.Exit(1)
		}

		signature := topic.name

		if topic.arguments != "" {
			signature += " " + topic.arguments
		}

		fmt.Printf("superstack %s\n\n  %s\n", signature, topic.summary)
		return
	}

	if entry.run == nil {
		fmt.Fprintf(os.Stderr, "superstack: %s is not implemented yet\n", entry.name)
		os.Exit(1)
	}

	err = commands.CheckServer()

	if err != nil {
		fmt.Fprintf(os.Stderr, "superstack: %s\n", err)
		os.Exit(1)
	}

	err = entry.run(rest)

	if err != nil {
		fmt.Fprintf(os.Stderr, "superstack: %s\n", err)
		os.Exit(1)
	}
}
