package dispatch

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/siliconwitchery/superstack-cli/internal/api"
)

type Command struct {
	Name      string
	Arguments string
	Summary   string
	Run       func(invocation api.Invocation, arguments []string) error
}

type Section struct {
	Title    string
	Commands []Command
}

func resolve(sections []Section, arguments []string) (Command, []string, bool) {
	longest := Command{}
	longestWords := 0

	for _, section := range sections {
		for _, candidate := range section.Commands {
			words := strings.Fields(candidate.Name)

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
		return Command{}, nil, false
	}

	return longest, arguments[longestWords:], true
}

func printHelp(invocation api.Invocation, sections []Section) {
	widest := 0

	for _, section := range sections {
		for _, entry := range section.Commands {
			width := len(entry.Name)

			if entry.Arguments != "" {
				width += 1 + len(entry.Arguments)
			}

			if width > widest {
				widest = width
			}
		}
	}

	fmt.Fprintf(invocation.Out, "superstack %s\n\n", invocation.Version)
	fmt.Fprint(invocation.Out, "Usage: superstack <command> [arguments]\n")

	for _, section := range sections {
		fmt.Fprintf(invocation.Out, "\n%s\n", section.Title)

		for _, entry := range section.Commands {
			signature := entry.Name

			if entry.Arguments != "" {
				signature += " " + entry.Arguments
			}

			fmt.Fprintf(invocation.Out, "  %-*s  %s\n", widest, signature, entry.Summary)
		}
	}
}

func Dispatch(sections []Section, version string, arguments []string, in io.Reader, out io.Writer) error {
	remaining := []string{}
	base := api.DefaultBase

	for index := 0; index < len(arguments); index++ {
		switch {
		case arguments[index] == "--server":
			if index+1 == len(arguments) {
				return errors.New("--server needs an address")
			}

			index++

			base = arguments[index]

		case strings.HasPrefix(arguments[index], "--server="):
			base = strings.TrimPrefix(arguments[index], "--server=")

		default:
			remaining = append(remaining, arguments[index])
		}
	}

	base = strings.TrimRight(strings.TrimSpace(base), "/")

	if base == "" {
		return errors.New("--server needs an address")
	}

	arguments = remaining

	invocation := api.NewInvocation(base, version, in, out)

	if len(arguments) == 0 {
		printHelp(invocation, sections)
		return nil
	}

	switch arguments[0] {
	case "-h", "--help":
		printHelp(invocation, sections)
		return nil

	case "-v", "--version":
		fmt.Fprintln(invocation.Out, invocation.Version)
		return nil
	}

	entry, rest, found := resolve(sections, arguments)

	if !found {
		return fmt.Errorf("unknown command %q\nRun 'superstack help' for the list.", strings.Join(arguments, " "))
	}

	switch entry.Name {
	case "version":
		fmt.Fprintln(invocation.Out, invocation.Version)
		return nil

	case "help":
		if len(rest) == 0 {
			printHelp(invocation, sections)
			return nil
		}

		topic, _, topicFound := resolve(sections, rest)

		if !topicFound {
			return fmt.Errorf("unknown command %q\nRun 'superstack help' for the list.", strings.Join(rest, " "))
		}

		signature := topic.Name

		if topic.Arguments != "" {
			signature += " " + topic.Arguments
		}

		fmt.Fprintf(invocation.Out, "superstack %s\n\n  %s\n", signature, topic.Summary)
		return nil
	}

	if entry.Run == nil {
		return fmt.Errorf("%s is not available yet", entry.Name)
	}

	return entry.Run(invocation, rest)
}
