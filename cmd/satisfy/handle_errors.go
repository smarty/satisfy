package main

import (
	"errors"
	"fmt"
	"iter"
	"os"

	"github.com/smarty/satisfy/contracts"
)

func handleCheck(seq iter.Seq2[contracts.Event, error]) {
	for event, err := range seq {
		if err != nil {
			if errors.Is(err, contracts.ErrPackageExists) {
				fmt.Fprintln(os.Stderr, "[INFO] Package already exists on remote storage.")
				os.Exit(2)
			}

			fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
			os.Exit(1)
		}

		printEvent(event)
	}
}

func handleDownload(seq iter.Seq2[contracts.Event, error]) {
	for event, err := range seq {
		if err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
			os.Exit(1)
		}

		printEvent(event)
	}
}

func handleUpload(seq iter.Seq2[contracts.Event, error]) {
	for event, err := range seq {
		if err != nil {
			if errors.Is(err, contracts.ErrPackageExists) {
				fmt.Fprintln(os.Stderr, "[INFO] Package already exists on remote storage.")
				os.Exit(2)
			}

			fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
			os.Exit(1)
		}

		printEvent(event)
	}
}

func handleParsing(seq iter.Seq2[contracts.Event, error]) {
	for event, err := range seq {
		printEvent(event)

		if err != nil {
			if errors.Is(err, contracts.ErrNoDependenciesMatch) {
				os.Exit(0)
			}

			if event.Message == "" {
				fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "[INFO] %v\n", err)
			}

			os.Exit(1)
		}
	}
}

func errSeq(err error) iter.Seq2[contracts.Event, error] {
	return func(yield func(contracts.Event, error) bool) {
		yield(contracts.Event{}, err)
	}
}

func eventErrSeq(event contracts.Event, err error) iter.Seq2[contracts.Event, error] {
	return func(yield func(contracts.Event, error) bool) {
		yield(event, err)
	}
}

func printEvent(event contracts.Event) {
	switch event.Type {
	case contracts.EventProgress:
		fmt.Println(event.Message)
	case contracts.EventInfo:
		fmt.Fprintf(os.Stderr, "[INFO] %s\n", event.Message)
	case contracts.EventWarning:
		fmt.Fprintf(os.Stderr, "[WARN] %s\n", event.Message)
	case contracts.EventFailure:
		fmt.Fprintf(os.Stderr, "[ERROR] %s\n", event.Message)
	}
}
