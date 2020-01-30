package core

import "bitbucket.org/smartystreets/satisfy/contracts"

func Uninstall(manifest contracts.Manifest, deleter contracts.Deleter) {
	for _, item := range manifest.Archive.Contents {
		deleter.Delete(item.Path)
	}
}
