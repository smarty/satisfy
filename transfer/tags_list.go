package transfer

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/smarty/gcs"

	"github.com/smarty/satisfy/contracts"
	"github.com/smarty/satisfy/core"
	"github.com/smarty/satisfy/shell"
)

type TagsListConfig struct {
	PackageName       string
	RemoteAddress     contracts.URL
	GoogleCredentials gcs.Credentials
	MaxRetry          int
	JSONOutput        bool
}

func ParseTagsListConfig(args []string) (config TagsListConfig, err error) {
	flags := flag.NewFlagSet("satisfy tags list", flag.ContinueOnError)

	var bucket, prefix, packageName string
	flags.StringVar(&bucket,
		"bucket",
		"",
		"GCS bucket name (required), e.g. liveaddress-downloads-dev.",
	)
	flags.StringVar(&prefix,
		"path",
		"",
		"Optional path prefix within the bucket where packages live, e.g. /releases.",
	)
	flags.StringVar(&packageName,
		"package",
		"",
		"Package name (required), e.g. master-address-list/2026/04/premium/az.",
	)
	flags.IntVar(&config.MaxRetry,
		"max-retry",
		5,
		"How many times to retry the manifest download.",
	)
	flags.BoolVar(&config.JSONOutput,
		"json",
		false,
		"Emit the tags as a JSON array instead of tab-separated lines.",
	)

	flags.Usage = func() {
		output := flags.Output()
		_, _ = fmt.Fprintf(output, "Usage: %s tags list -bucket <name> [-path <prefix>] -package <name> [-json]\n\n", os.Args[0])
		flags.PrintDefaults()
	}

	if err = flags.Parse(args); err != nil {
		return TagsListConfig{}, err
	}

	if bucket == "" {
		return TagsListConfig{}, errors.New("-bucket is required")
	}
	if strings.Contains(bucket, "/") {
		return TagsListConfig{}, errors.New("-bucket should be a bare bucket name (no scheme, no slashes)")
	}

	config.PackageName = strings.Trim(packageName, "/")
	if config.PackageName == "" {
		return TagsListConfig{}, errors.New("-package is required")
	}

	config.RemoteAddress = contracts.URL{Scheme: "gcs", Host: bucket}
	if prefix = strings.Trim(prefix, "/"); prefix != "" {
		config.RemoteAddress.Path = "/" + prefix
	}

	reader := gcs.NewCredentialsReader()
	config.GoogleCredentials, err = reader.Read(context.Background(), "")
	if err != nil {
		return TagsListConfig{}, fmt.Errorf("could not load Google credentials: %w", err)
	}
	return config, nil
}

type TagsListApp struct {
	config TagsListConfig
	output io.Writer
}

func NewTagsListApp(config TagsListConfig) *TagsListApp {
	return &TagsListApp{config: config, output: os.Stdout}
}

func (this *TagsListApp) Run() {
	if err := this.TryRun(); err != nil {
		log.Fatal(err)
	}
}

func (this *TagsListApp) TryRun() error {
	gcsClient := shell.NewGoogleCloudStorageClient(shell.NewHTTPClient(), this.config.GoogleCredentials, []int{http.StatusOK})
	client := core.NewRetryClient(gcsClient, this.config.MaxRetry, time.Sleep)

	dependency := contracts.Dependency{
		PackageName:   this.config.PackageName,
		RemoteAddress: this.config.RemoteAddress,
	}

	address := dependency.ComposeLatestManifestRemoteAddress()
	body, err := client.Download(address)
	if contracts.IsNotFound(err) {
		return fmt.Errorf("no root manifest found for package %q at [%s] (has the package been uploaded?)",
			this.config.PackageName, address.String())
	}
	if err != nil {
		return fmt.Errorf("could not download root manifest for %q: %w", this.config.PackageName, err)
	}
	defer func() { _ = body.Close() }()

	var manifest contracts.Manifest
	if err = json.NewDecoder(body).Decode(&manifest); err != nil {
		return fmt.Errorf("could not decode manifest: %w", err)
	}

	formatted, err := FormatTags(manifest.Tags, this.config.JSONOutput)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(this.output, formatted)
	return err
}

// FormatTags renders a tag list for the `tags list` command. With asJSON it
// emits a JSON array (an empty list renders as "[]"). Otherwise it emits one
// "name<TAB>version" line per tag, sorted by name; an empty list renders as the
// empty string so that scripts see no output and a zero exit code.
func FormatTags(tags []contracts.Tag, asJSON bool) (string, error) {
	sorted := append([]contracts.Tag(nil), tags...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	if asJSON {
		if sorted == nil {
			sorted = []contracts.Tag{}
		}
		raw, err := json.MarshalIndent(sorted, "", "  ")
		if err != nil {
			return "", err
		}
		return string(raw) + "\n", nil
	}

	builder := new(strings.Builder)
	for _, tag := range sorted {
		_, _ = fmt.Fprintf(builder, "%s\t%s\n", tag.Name, tag.Version)
	}
	return builder.String(), nil
}
