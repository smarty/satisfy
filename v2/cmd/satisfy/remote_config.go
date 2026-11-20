package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/smarty/satisfy/v2"
)

// remotePackageFlags are the flags shared by commands that look up a single
// package's root manifest (latest, tags list).
type remotePackageFlags struct {
	bucket      string
	prefix      string
	packageName string
	maxRetry    int
}

func (this *remotePackageFlags) register(flags *flag.FlagSet) {
	flags.StringVar(&this.bucket,
		"bucket",
		"",
		"GCS bucket name (required), e.g. liveaddress-downloads-dev.",
	)
	flags.StringVar(&this.prefix,
		"path",
		"",
		"Optional path prefix within the bucket where packages live, e.g. /releases.",
	)
	flags.StringVar(&this.packageName,
		"package",
		"",
		"Package name (required), e.g. master-address-list/2026/04/premium/az.",
	)
	flags.IntVar(&this.maxRetry,
		"max-retry",
		5,
		"How many times to retry the manifest download.",
	)
}

// resolve validates the flags and returns the package they identify.
func (this *remotePackageFlags) resolve() (ref satisfy.PackageRef, err error) {
	if this.bucket == "" {
		return satisfy.PackageRef{}, errors.New("-bucket is required")
	}
	if strings.Contains(this.bucket, "/") {
		return satisfy.PackageRef{}, errors.New("-bucket should be a bare bucket name (no scheme, no slashes)")
	}

	ref.Name = strings.Trim(this.packageName, "/")
	if ref.Name == "" {
		return satisfy.PackageRef{}, errors.New("-package is required")
	}

	ref.RemoteAddress = satisfy.URL{Scheme: "gcs", Host: this.bucket}
	if prefix := strings.Trim(this.prefix, "/"); prefix != "" {
		ref.RemoteAddress.Path = "/" + prefix
	}
	return ref, nil
}

// remotePackageConfig identifies one package's root manifest.
type remotePackageConfig struct {
	Package  satisfy.PackageRef
	MaxRetry int
}

func parseLatestConfig(args []string, stderr io.Writer) (config remotePackageConfig, err error) {
	flags := flag.NewFlagSet("satisfy latest", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var remote remotePackageFlags
	remote.register(flags)
	flags.Usage = func() {
		_, _ = fmt.Fprintln(flags.Output(), "Usage: satisfy latest -bucket <name> [-path <prefix>] -package <name>")
		_, _ = fmt.Fprintln(flags.Output())
		flags.PrintDefaults()
	}
	if err = flags.Parse(args); err != nil {
		return remotePackageConfig{}, err
	}
	config.Package, err = remote.resolve()
	if err != nil {
		return remotePackageConfig{}, err
	}
	config.MaxRetry = remote.maxRetry
	return config, nil
}

// tagsListCommandConfig is the configuration for the tags list command.
type tagsListCommandConfig struct {
	remotePackageConfig
	JSONOutput bool
}

func parseTagsListConfig(args []string, stderr io.Writer) (config tagsListCommandConfig, err error) {
	flags := flag.NewFlagSet("satisfy tags list", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var remote remotePackageFlags
	remote.register(flags)
	flags.BoolVar(&config.JSONOutput,
		"json",
		false,
		"Emit the tags as a JSON array instead of tab-separated lines.",
	)
	flags.Usage = func() {
		_, _ = fmt.Fprintln(flags.Output(), "Usage: satisfy tags list -bucket <name> [-path <prefix>] -package <name> [-json]")
		_, _ = fmt.Fprintln(flags.Output())
		flags.PrintDefaults()
	}
	if err = flags.Parse(args); err != nil {
		return tagsListCommandConfig{}, err
	}
	config.Package, err = remote.resolve()
	if err != nil {
		return tagsListCommandConfig{}, err
	}
	config.MaxRetry = remote.maxRetry
	return config, nil
}
