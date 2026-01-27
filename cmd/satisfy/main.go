package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"

	"github.com/smarty/gcs"
	"github.com/smarty/satisfy/configuration"
	"github.com/smarty/satisfy/contracts"
	"github.com/smarty/satisfy/logging"
	"github.com/smarty/satisfy/shell"
	"github.com/smarty/satisfy/transfer"
)

const messageDownloadIsDefault = "Note: 'download' is the default command and doesn't need to be specified."

var helpFlags = []string{"-h", "--help", "-help"}

var logger = logging.NewLogger(os.Stdout, os.Stderr, os.Exit)

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

func downloadDependencyListFunc(path string) (listing contracts.DependencyListing, err error) {
	if path == configuration.StdInPath {
		return readFromReader(os.Stdin)
	} else {
		file, err := os.Open(path)
		if os.IsNotExist(err) {
			configuration.EmitExampleDependenciesFile(logger)
			return listing, fmt.Errorf("specified dependency file (%q) not found: %w", path, err)
		}

		if err != nil {
			return listing, fmt.Errorf("could not open specified dependency file (%q): %w", path, err)
		}

		defer func() { _ = file.Close() }()
		return readFromReader(file)
	}
}

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
	config := configuration.NewCheckConfiguration(
		context.Background(),
		readPackageConfigFunc,
		gcs.NewCredentialsReader(
			gcs.CredentialOptions.VaultServer(os.Getenv("VAULT_ADDR"), os.Getenv("VAULT_TOKEN")),
			gcs.CredentialOptions.EnvironmentReader(shell.NewEnvironment()),
			gcs.CredentialOptions.FileReader(shell.NewDiskFileSystem("")),
		),
		logger,
	)
	err := config.Parse(args)
	if err != nil {
		logger.Fatal(err)
	}

	transfer.NewCheckApp(*config).Run()
}

func mainDownload(args []string) {
	config := configuration.NewDownloadConfiguration(
		context.Background(),
		downloadDependencyListFunc,
		gcs.NewCredentialsReader(),
		logger,
	)
	err := config.Parse(args)
	if err != nil {
		logger.Fatal(err)
	}

	transfer.NewDownloadApp(*config).Run()
}

func mainUpload(args []string) {
	config := configuration.NewUploadConfiguration(
		context.Background(),
		readPackageConfigFunc,
		gcs.NewCredentialsReader(
			gcs.CredentialOptions.VaultServer(os.Getenv("VAULT_ADDR"), os.Getenv("VAULT_TOKEN")),
			gcs.CredentialOptions.EnvironmentReader(shell.NewEnvironment()),
			gcs.CredentialOptions.FileReader(shell.NewDiskFileSystem("")),
		),
		logger,
	)
	err := config.Parse(args)
	if err != nil {
		logger.FatalWithLevel(logging.Info, err)
	}

	transfer.NewUploadApp(*config).Run()
}

func mainVersion() {
	logger.LogLine(logging.Info, "satisfy [debug]")
}

func printAvailableCommands() {
	logger.LogLineClean("Available commands:\n")
	for _, cmd := range validCommands {
		logger.LogLineClean("  %-12s %s", cmd.Name, cmd.Description)
		logger.LogLineClean("  %-12s Usage: %s\n", "", cmd.Usage)
	}

	logger.LogLineClean("%s", messageDownloadIsDefault)
}

func readFromReader(reader io.Reader) (listing contracts.DependencyListing, err error) {
	decoder := json.NewDecoder(reader)
	err = decoder.Decode(&listing)
	return listing, err
}

func readPackageConfigFunc(path string) (config contracts.PackageConfig, err error) {
	var data []byte
	if path == configuration.StdInPath {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path)
	}

	if err != nil {
		return config, fmt.Errorf("could not read config file (%q): %w", path, err)
	}

	err = json.Unmarshal(data, &config)
	if err != nil {
		return config, fmt.Errorf("could not parse config file (%q): %w", path, err)
	}

	return config, nil
}
