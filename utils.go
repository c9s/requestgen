package main

import (
	"os"

	"github.com/sirupsen/logrus"
)

// isDirectory reports whether the named file is a directory.
func isDirectory(name string) bool {
	info, err := os.Stat(name)
	if err != nil {
		logrus.Fatal(err)
	}
	return info.IsDir()
}

