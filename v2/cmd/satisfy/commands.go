package main

import (
	"context"
	"fmt"

	"github.com/smarty/gcs"

	"github.com/smarty/satisfy/v2"
	"github.com/smarty/satisfy/v2/internal/fsys"
)

func (this environment) client(options ...satisfy.Option) *satisfy.Client {
	return satisfy.New(append([]satisfy.Option{satisfy.WithLogger(this.logger)}, options...)...)
}

func uploadCommand(ctx context.Context, env environment, args []string) error {
	config, err := newUploadConfigLoader(env).LoadConfig(ctx, "upload", args)
	if err != nil {
		return err
	}
	client := env.client(
		// compressing a large package can outlive a bearer token, so read
		// credentials again as they age
		satisfy.WithCredentialsReader(config.CredentialReader),
		satisfy.WithMaxRetry(config.MaxRetry),
		satisfy.WithProgress(env.progress(config.ShowProgress)),
	)
	if config.Overwrite {
		env.logger.Info("overwrite mode enabled; skipping remote manifest check")
	}
	_, err = client.Upload(ctx, satisfy.UploadRequest{Package: config.PackageConfig, Overwrite: config.Overwrite})
	return err
}

func checkCommand(ctx context.Context, env environment, args []string) error {
	config, err := newUploadConfigLoader(env).LoadConfig(ctx, "check", args)
	if err != nil {
		return err
	}
	if config.Overwrite {
		env.logger.Info("overwrite mode enabled; skipping remote manifest check")
		return nil
	}
	client := env.client(satisfy.WithCredentialsReader(config.CredentialReader), satisfy.WithMaxRetry(config.MaxRetry))
	pkg := config.PackageConfig
	exists, err := client.Exists(ctx, satisfy.PackageRef{RemoteAddress: pkg.RemoteAddress, Name: pkg.PackageName}, pkg.PackageVersion)
	if err != nil {
		return fmt.Errorf("sanity check failed: %w", err)
	}
	if exists {
		return fmt.Errorf("%s @ %s: %w", pkg.PackageName, pkg.PackageVersion, satisfy.ErrPackageExists)
	}
	return nil
}

func newUploadConfigLoader(env environment) *UploadConfigLoader {
	return NewUploadConfigLoader(fsys.NewDiskFileSystem(""), newOSEnvironment(), env.stdin, env.stderr)
}

func downloadCommand(ctx context.Context, env environment, args []string) error {
	config, err := parseDownloadConfig(args, env.stdin, env.stderr)
	if err != nil {
		return err
	}
	if len(config.Dependencies.Listing) == 0 {
		return nil
	}
	options := []satisfy.Option{
		satisfy.WithMaxRetry(config.MaxRetry),
		satisfy.WithConcurrency(config.Concurrency),
		satisfy.WithContentVerification(!config.QuickVerification),
		satisfy.WithProgress(env.progress(config.ShowProgress)),
	}
	// Without an explicit access token in the listing, the client discovers
	// credentials from the environment and reads them again as they age, so
	// that long installs outlive any one bearer token.
	if token := config.Dependencies.Credentials; token != "" {
		credentials, err := readCredentials(ctx, token)
		if err != nil {
			return err
		}
		options = append(options, satisfy.WithCredentials(credentials))
	}
	return env.client(options...).Install(ctx, config.Dependencies.Listing...)
}

func latestCommand(ctx context.Context, env environment, args []string) error {
	config, err := parseLatestConfig(args, env.stderr)
	if err != nil {
		return err
	}
	client := env.client(satisfy.WithMaxRetry(config.MaxRetry)) // credentials from the environment
	version, err := client.Latest(ctx, config.Package)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(env.stdout, version)
	return err
}

func tagsListCommand(ctx context.Context, env environment, args []string) error {
	config, err := parseTagsListConfig(args, env.stderr)
	if err != nil {
		return err
	}
	client := env.client(satisfy.WithMaxRetry(config.MaxRetry)) // credentials from the environment
	tags, err := client.Tags(ctx, config.Package)
	if err != nil {
		return err
	}
	formatted, err := formatTags(tags, config.JSONOutput)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(env.stdout, formatted)
	return err
}

func tagsModifyCommand(ctx context.Context, env environment, args []string) error {
	loader := NewTagsConfigLoader(fsys.NewDiskFileSystem(""), newOSEnvironment(), env.stdin, env.stderr)
	config, err := loader.LoadConfig(ctx, args)
	if err != nil {
		return err
	}
	client := env.client(satisfy.WithCredentialsReader(config.CredentialReader), satisfy.WithMaxRetry(config.MaxRetry))
	return client.ModifyTags(ctx, config.Modification)
}

// readCredentials returns the explicit access token as credentials.
func readCredentials(ctx context.Context, token string) (gcs.Credentials, error) {
	credentials, err := gcs.NewCredentialsReader().Read(ctx, token)
	if err != nil {
		return gcs.Credentials{}, fmt.Errorf("could not load Google credentials: %w", err)
	}
	return credentials, nil
}
