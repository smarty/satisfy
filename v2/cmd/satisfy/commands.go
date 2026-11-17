package main

import (
	"context"
	"fmt"

	"github.com/smarty/gcs"

	"github.com/smarty/satisfy/v2/shell"
	"github.com/smarty/satisfy/v2/transfer"
)

func uploadCommand(ctx context.Context, env environment, args []string) error {
	config, err := newUploadConfigLoader(env).LoadConfig(ctx, "upload", args)
	if err != nil {
		return err
	}
	config.Logger = env.logger
	config.Progress = env.progress(config.ShowProgress)
	if err = transfer.NewCheckApp(config.UploadConfig).Run(ctx); err != nil {
		return err
	}
	return transfer.NewUploadApp(config.UploadConfig).Run(ctx)
}

func checkCommand(ctx context.Context, env environment, args []string) error {
	config, err := newUploadConfigLoader(env).LoadConfig(ctx, "check", args)
	if err != nil {
		return err
	}
	config.Logger = env.logger
	return transfer.NewCheckApp(config.UploadConfig).Run(ctx)
}

func newUploadConfigLoader(env environment) *UploadConfigLoader {
	return NewUploadConfigLoader(shell.NewDiskFileSystem(""), shell.NewEnvironment(), env.stdin, env.stderr)
}

func downloadCommand(ctx context.Context, env environment, args []string) error {
	parsed, err := parseDownloadConfig(args, env.stdin, env.stderr)
	if err != nil {
		return err
	}
	config := parsed.DownloadConfig
	if len(config.Dependencies.Listing) == 0 {
		return nil
	}
	config.GoogleCredentials, err = readCredentials(ctx, parsed.Dependencies.Credentials)
	if err != nil {
		return err
	}
	config.Logger = env.logger
	config.Progress = env.progress(parsed.ShowProgress)
	return transfer.NewDownloadApp(config).Run(ctx)
}

func latestCommand(ctx context.Context, env environment, args []string) error {
	config, err := parseLatestConfig(args, env.stderr)
	if err != nil {
		return err
	}
	if config.GoogleCredentials, err = readCredentials(ctx, ""); err != nil {
		return err
	}
	config.Logger = env.logger
	version, err := transfer.NewLatestApp(config).Run(ctx)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(env.stdout, version)
	return err
}

func tagsListCommand(ctx context.Context, env environment, args []string) error {
	parsed, err := parseTagsListConfig(args, env.stderr)
	if err != nil {
		return err
	}
	config := parsed.TagsListConfig
	if config.GoogleCredentials, err = readCredentials(ctx, ""); err != nil {
		return err
	}
	config.Logger = env.logger
	tags, err := transfer.NewTagsListApp(config).Run(ctx)
	if err != nil {
		return err
	}
	formatted, err := formatTags(tags, parsed.JSONOutput)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(env.stdout, formatted)
	return err
}

func tagsModifyCommand(ctx context.Context, env environment, args []string) error {
	loader := NewTagsConfigLoader(shell.NewDiskFileSystem(""), shell.NewEnvironment(), env.stdin, env.stderr)
	config, err := loader.LoadConfig(ctx, args)
	if err != nil {
		return err
	}
	config.Logger = env.logger
	return transfer.NewTagsApp(config.TagsConfig).Run(ctx)
}

// readCredentials discovers Google credentials from the environment, or uses
// the explicit access token when one is given.
func readCredentials(ctx context.Context, token string) (gcs.Credentials, error) {
	credentials, err := gcs.NewCredentialsReader().Read(ctx, token)
	if err != nil {
		return gcs.Credentials{}, fmt.Errorf("could not load Google credentials: %w", err)
	}
	return credentials, nil
}
