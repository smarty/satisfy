package main

import (
	"errors"
	"os"
	"slices"
	"sort"

	satisfy "github.com/smarty/satisfy"
	"github.com/smarty/satisfy/contracts"
)

const messageDownloadIsDefault = "Note: 'download' is the default command and doesn't need to be specified."

var helpFlags = []string{"-h", "--help", "-help"}

var logger = contracts.NewLogger(os.Stdout, os.Stderr, os.Exit)

var validCommands = []Command{
	{
		Name:        "check",
		Description: "Check if a package version exists on remote storage",
		Usage:       "satisfy check [-json=<config.json>] [-max-retry=5]",
		Function: func() {
			mainCheck(os.Args[2:])
		},
	},
	{
		Name:        "download",
		Description: "Download and install package dependencies (default)",
		Usage:       "satisfy [-json=<deps.json>] [-max-retry=5] [-quick] [-progress]",
		Function: func() {
			logger.LogLineClean("%s\n", messageDownloadIsDefault)
			mainDownload(os.Args[2:])
		},
	},
	{
		Name:        "upload",
		Description: "Upload a package archive to remote storage",
		Usage:       "satisfy upload [-json=<config.json>] [-max-retry=5] [-overwrite] [-progress]",
		Function: func() {
			mainUpload(os.Args[2:])
		},
	},
	{
		Name:        "version",
		Description: "Display the satisfy version",
		Usage:       "satisfy version",
		Function: func() {
			mainVersion()
		},
	},
}

func main() {
	if len(os.Args) == 1 {
		mainDownload([]string{})
		return
	}

	if isHelpFlag(os.Args[1]) {
		printAvailableCommands()
		return
	}

	if looksLikeFlag(os.Args[1]) {
		mainDownload(os.Args[1:])
		return
	}

	subCommand := os.Args[1]
	for _, command := range validCommands {
		if subCommand == command.Name {
			command.Function()
			return
		}
	}

	handleMalformedSubcommand(os.Args[1])
	os.Exit(1)
}

// ----- helper functions -----

func handleMalformedSubcommand(input string) {
	const threshold = 2

	logger.LogLineClean("Error: Unknown subcommand '%s'\n", input)

	var suggestions []suggestion
	for _, command := range validCommands {
		distance := levenshteinDistance(input, command.Name)
		suggestions = append(suggestions, suggestion{Command: command, Distance: distance})
	}

	sort.Slice(suggestions, func(iLeft, iRight int) bool {
		return suggestions[iLeft].Distance < suggestions[iRight].Distance
	})

	closestDistance := suggestions[0].Distance
	if closestDistance <= threshold {
		logger.LogLineClean("Did you mean '%s'?\n", suggestions[0].Command.Name)
	}

	printAvailableCommands()
}

func isHelpFlag(arg string) bool {
	return slices.Contains(helpFlags, arg)
}

func levenshteinDistance(left, right string) int {
	if len(left) == 0 {
		return len(right)
	}

	if len(right) == 0 {
		return len(left)
	}

	matrix := make([][]int, len(left)+1)
	for iRow := range matrix {
		matrix[iRow] = make([]int, len(right)+1)
	}

	for iRow := 0; iRow <= len(left); iRow++ {
		matrix[iRow][0] = iRow
	}

	for iColumn := 1; iColumn <= len(right); iColumn++ {
		matrix[0][iColumn] = iColumn
	}

	for iRow := 1; iRow <= len(left); iRow++ {
		for iColumn := 1; iColumn <= len(right); iColumn++ {
			cost := 0
			if left[iRow-1] != right[iColumn-1] {
				cost = 1
			}

			matrix[iRow][iColumn] = min(
				matrix[iRow-1][iColumn]+1,      // deletion
				matrix[iRow][iColumn-1]+1,      // insertion
				matrix[iRow-1][iColumn-1]+cost, // substitution
			)
		}
	}

	return matrix[len(left)][len(right)]
}

func looksLikeFlag(arg string) bool {
	return len(arg) > 0 && arg[0] == '-'
}

func mainCheck(args []string) {
	config, err := parseCheck(args)
	if err != nil {
		logger.Fatal(err)
	}

	satisfy.Check(config)
}

func mainDownload(args []string) {
	config, err := parseDownload(args)
	if errors.Is(err, contracts.ErrNoDependenciesMatch) {
		return
	}

	if err != nil {
		logger.Fatal(err)
	}

	satisfy.Download(config)
}

func mainUpload(args []string) {
	config, err := parseUpload(args)
	if err != nil {
		logger.FatalWithLevel(contracts.Info, err)
	}

	satisfy.Upload(config)
}

func mainVersion() {
	logger.LogLine(contracts.Info, "satisfy [debug]")
}

func printAvailableCommands() {
	logger.LogLineClean("Available commands:\n")
	for _, cmd := range validCommands {
		logger.LogLineClean("  %-12s %s", cmd.Name, cmd.Description)
		logger.LogLineClean("  %-12s Usage: %s\n", "", cmd.Usage)
	}

	logger.LogLineClean("%s", messageDownloadIsDefault)
}
