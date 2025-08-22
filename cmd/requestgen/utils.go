package main

import (
	"bytes"
	"go/format"
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

func formatBuffer(buf bytes.Buffer) []byte {
	logrus.Infof("formatting source code with %d bytes...", buf.Len())

	p := newProfile("formatSource")
	defer p.stop()

	src, err := format.Source(buf.Bytes())
	if err != nil {
		// Should never happen, but can arise when developing this code.
		// The user can compile the output to see the error.
		logrus.Errorf("internal error: invalid Go generated: %s", err)
		logrus.Error("please compile the package to analyze the error")
		return buf.Bytes()
	}

	return src
}
