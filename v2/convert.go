package satisfy

import (
	"strings"

	"github.com/smarty/satisfy/v2/internal/model"
)

// The public types mirror types in internal/model. These functions copy
// values across that boundary; convert_test.go checks that every field is
// copied.

func (this PackageRef) internal() model.PackageRef {
	return model.PackageRef{RemoteAddress: model.URL(this.RemoteAddress), Name: strings.Trim(this.Name, "/")}
}

func (this Dependency) internal() model.Dependency {
	return model.Dependency{
		PackageName:    this.PackageName,
		PackageVersion: this.PackageVersion,
		RemoteAddress:  model.URL(this.RemoteAddress),
		LocalDirectory: this.LocalDirectory,
	}
}

func (this PackageConfig) internal() model.PackageConfig {
	return model.PackageConfig{
		CompressionAlgorithm: this.CompressionAlgorithm,
		CompressionLevel:     this.CompressionLevel,
		SourceDirectory:      this.SourceDirectory,
		SourceFile:           this.SourceFile,
		SourcePath:           this.SourcePath,
		PackageName:          this.PackageName,
		PackageVersion:       this.PackageVersion,
		RemoteAddress:        model.URL(this.RemoteAddress),
		Tags:                 this.Tags,
	}
}

func (this TagModification) internal() model.TagModificationConfig {
	return model.TagModificationConfig{
		PackageName:   this.PackageName,
		RemoteAddress: model.URL(this.RemoteAddress),
		Add:           internalTags(this.Add),
		Delete:        internalTags(this.Delete),
	}
}

func internalTags(tags []Tag) []model.Tag {
	if tags == nil {
		return nil
	}
	converted := make([]model.Tag, 0, len(tags))
	for _, tag := range tags {
		converted = append(converted, model.Tag(tag))
	}
	return converted
}

func publicTags(tags []model.Tag) []Tag {
	if tags == nil {
		return nil
	}
	converted := make([]Tag, 0, len(tags))
	for _, tag := range tags {
		converted = append(converted, Tag(tag))
	}
	return converted
}

func publicManifest(manifest model.Manifest) Manifest {
	return Manifest{
		Name:    manifest.Name,
		Version: manifest.Version,
		Archive: Archive{
			Filename:             manifest.Archive.Filename,
			Size:                 manifest.Archive.Size,
			MD5Checksum:          manifest.Archive.MD5Checksum,
			Contents:             publicArchiveItems(manifest.Archive.Contents),
			CompressionAlgorithm: manifest.Archive.CompressionAlgorithm,
		},
		Tags: publicTags(manifest.Tags),
	}
}

func publicArchiveItems(items []model.ArchiveItem) []ArchiveItem {
	if items == nil {
		return nil
	}
	converted := make([]ArchiveItem, 0, len(items))
	for _, item := range items {
		converted = append(converted, ArchiveItem(item))
	}
	return converted
}

func publicProgress(progress model.Progress) Progress {
	return Progress{
		Phase:   ProgressPhase(progress.Phase),
		Name:    progress.Name,
		Written: progress.Written,
		Total:   progress.Total,
		Done:    progress.Done,
	}
}
