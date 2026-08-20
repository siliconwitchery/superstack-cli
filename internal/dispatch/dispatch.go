package dispatch

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/siliconwitchery/superstack-cli/internal/api"
)

func takeServerFlag(arguments []string) ([]string, string, error) {
	remaining := []string{}

	base := api.DefaultBase

	for index := 0; index < len(arguments); index++ {
		switch {
		case arguments[index] == "--server":
			if index+1 == len(arguments) {
				return nil, "", errors.New("--server needs an address")
			}

			index++

			base = arguments[index]

		case strings.HasPrefix(arguments[index], "--server="):
			base = strings.TrimPrefix(arguments[index], "--server=")

		default:
			remaining = append(remaining, arguments[index])
		}
	}

	base = strings.TrimRight(base, "/")

	if base == "" {
		return nil, "", errors.New("--server needs an address")
	}

	return remaining, base, nil
}

type Command struct {
	Name      string
	Arguments string
	Summary   string
	Run       func(session api.Session, arguments []string) error
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

func printHelp(session api.Session, sections []Section) {
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

	fmt.Fprintf(session.Out, "superstack %s\n\n", session.Version)
	fmt.Fprint(session.Out, "Usage: superstack <command> [arguments]\n")

	for _, section := range sections {
		fmt.Fprintf(session.Out, "\n%s\n", section.Title)

		for _, entry := range section.Commands {
			signature := entry.Name

			if entry.Arguments != "" {
				signature += " " + entry.Arguments
			}

			fmt.Fprintf(session.Out, "  %-*s  %s\n", widest, signature, entry.Summary)
		}
	}
}

func Dispatch(sections []Section, version string, arguments []string, in io.Reader, out io.Writer) error {
	arguments, base, err := takeServerFlag(arguments)

	if err != nil {
		return err
	}

	session := api.NewSession(base, version, in, out)

	if len(arguments) == 0 {
		printHelp(session, sections)
		return nil
	}

	switch arguments[0] {
	case "-h", "--help":
		printHelp(session, sections)
		return nil

	case "-v", "--version":
		fmt.Fprintln(session.Out, session.Version)
		return nil
	}

	entry, rest, found := resolve(sections, arguments)

	if !found {
		return fmt.Errorf("unknown command %q\nRun 'superstack help' for the list.", strings.Join(arguments, " "))
	}

	switch entry.Name {
	case "version":
		fmt.Fprintln(session.Out, session.Version)
		return nil

	case "help":
		if len(rest) == 0 {
			printHelp(session, sections)
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

		fmt.Fprintf(session.Out, "superstack %s\n\n  %s\n", signature, topic.Summary)
		return nil
	}

	if entry.Run == nil {
		return fmt.Errorf("%s is not available yet", entry.Name)
	}

	err = api.CheckServer(session)

	if err != nil {
		return err
	}

	err = entry.Run(session, rest)

	return err
}
