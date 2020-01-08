package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

func main() {

	err := filepath.Walk("/Users/Gordon/src/bitbucket.org", walker)
	if err != nil {
		log.Fatal(err)
	}
}

func walker(path string, info os.FileInfo, err error) error {
	if info.Name() == ".git" && info.IsDir() {
		return filepath.SkipDir
	}
	fmt.Println(path, info.IsDir())
	return errors.New("Hello")
}

func readDir() {
	dir, err := ioutil.ReadDir("/Users/Gordon/src/bitbucket.org/smartystreets")
	if err != nil {
		log.Fatal(err)
	}
	for _, item := range dir {
		fmt.Println(item.Name(), item.IsDir())
	}
}
