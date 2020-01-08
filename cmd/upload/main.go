package main

import (
	"crypto/md5"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/mholt/archiver"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

func main() {
	root := "/Users/Gordon/Desktop/bowling"
	manifest := createManifest("bowling-game", root, "1.2.3")
	manifestRoot := filepath.Join(root, "manifest")
	file, _ := os.Create(manifestRoot)
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "    ")
	encoder.Encode(manifest)
	sources := []string{}
	for _,contents := range manifest.Contents {
		sources = append(sources, filepath.Join(root, contents.Path))
	}

	gz := archiver.NewTarGz()
	gz.Archive(sources, filepath.Join(filepath.Dir(root), "bowling-game_1.2.3.tar.gz"))
}

func createManifest(name, root, version string) (manifest contracts.Manifest) {
	manifest.Name = name
	manifest.Version = version
	hasher := md5.New()

	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if (info.Name() == ".git" || info.Name() == ".idea") && info.IsDir() {
			return filepath.SkipDir
		}
		if info.IsDir() || info.Name() == "manifest" {
			return nil
		}
		raw, _ := ioutil.ReadFile(path)
		hasher.Reset()
		_, _ = hasher.Write(raw)
		manifest.Contents = append(manifest.Contents, contracts.FileInfo{
			Path:        path[len(root)+1:],
			Size:        info.Size(),
			MD5Checksum: hasher.Sum(nil),
		})
		return nil
	})
	return manifest
}
