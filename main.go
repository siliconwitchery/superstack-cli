package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/siliconwitchery/superstack-cli/internal/commands"
)

const version = "0.0.2"

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
			{name: "login", arguments: "<provider>", summary: "Log in with the selected provider", run: commands.Login},
			{name: "logout", summary: "Log out of your account", run: commands.Logout},
		},
	},
	{
		title: "Fleets",
		commands: []command{
			{name: "fleet create", arguments: "<name>", summary: "Create a fleet"},
			{name: "fleet list", summary: "List the fleets you can reach"},
			{name: "fleet rename", arguments: "<name>", summary: "Rename a fleet"},
			{name: "fleet delete", summary: "Delete a fleet and release its devices"},
		},
	},
	{
		title: "People",
		commands: []command{
			{name: "member add", arguments: "<email>", summary: "Give someone access to a fleet"},
			{name: "member list", summary: "List the people who can reach a fleet"},
			{name: "member remove", arguments: "<email>", summary: "Take away someone's access"},
		},
	},
	{
		title: "Devices",
		commands: []command{
			{name: "claim", arguments: "<imei> [name]", summary: "Claim a device into a fleet, then press its button"},
			{name: "devices", summary: "List devices, their state, and when they were last seen"},
			{name: "rename", arguments: "<name>", summary: "Rename a device"},
			{name: "release", summary: "Wipe a device and hand it back"},
			{name: "start", summary: "Run the code on the target"},
			{name: "stop", summary: "Halt the code on the target"},
			{name: "restart", summary: "Restart the code on the target"},
		},
	},
	{
		title: "Files",
		commands: []command{
			{name: "upload", arguments: "<path>", summary: "Upload a file or directory to the target"},
			{name: "download", arguments: "<path>", summary: "Download the target's files into <path>"},
			{name: "dev", arguments: "<path>", summary: "Upload on every change, and tail"},
		},
	},
	{
		title: "Data",
		commands: []command{
			{name: "tail", summary: "Stream events and logs as they arrive"},
			{name: "send", arguments: "-m <message>", summary: "Queue a message for the target to collect"},
		},
	},
	{
		title: "Account",
		commands: []command{
			{name: "balance", summary: "Show the credit left on your account"},
			{name: "topup", summary: "Add credit"},
			{name: "billing", summary: "Open your billing portal"},
		},
	},
	{
		title: "Superstack",
		commands: []command{
			{name: "upgrade", summary: "Replace this binary with the latest release"},
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
Targets
  Every command that touches devices takes --fleet or --device. With neither,
  it acts on your only fleet, and stops if you can reach more than one.

Flags
  --fleet <id>     Act on every device in this fleet
  --device <name>  Act on one device, by name or by IMEI
  --role <role>    admin or member, when adding someone to a fleet
  --yes            Do not ask before acting on more than one device
  --json           Print machine-readable output

Environment
  SUPERSTACK_API  Talk to this server instead of the production one
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
