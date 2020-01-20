package core

import "bitbucket.org/smartystreets/satisfy/contracts"

func Uninstall(manifest contracts.Manifest, delete func(string)) {
	for _, item := range manifest.Archive.Contents {
		delete(item.Path)
	}
}
