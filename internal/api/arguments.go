package api

func TakeJsonFlag(arguments []string) ([]string, bool) {
	positionals := []string{}
	jsonOutput := false

	for _, argument := range arguments {
		if argument == "--json" {
			jsonOutput = true
			continue
		}

		positionals = append(positionals, argument)
	}

	return positionals, jsonOutput
}
