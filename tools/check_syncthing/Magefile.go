//go:build mage

// Magefile for check_syncthing, the Syncthing Nagios/Gogios plugin.
// Targets follow the same style as the other snonux Go projects.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

const binaryName = "check_syncthing"

// Default builds the binary for the current platform.
func Default() { mg.Deps(Build) }

// Build builds ./check_syncthing for the current platform.
func Build() error {
	return sh.RunV("go", "build", "-o", binaryName, "./cmd/"+binaryName)
}

// BuildOpenBSD cross-builds ./check_syncthing.openbsd-amd64 for the frontends.
func BuildOpenBSD() error {
	env := map[string]string{"GOOS": "openbsd", "GOARCH": "amd64", "CGO_ENABLED": "0"}
	return sh.RunWithV(env, "go", "build", "-trimpath", "-o", binaryName+".openbsd-amd64", "./cmd/"+binaryName)
}

// Test runs the unit tests.
func Test() error {
	return sh.RunV("go", "test", "./...")
}

// Install copies the binary to $GOPATH/bin (default ~/go/bin).
func Install() error {
	mg.Deps(Build)
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve home: %w", err)
		}
		gopath = filepath.Join(home, "go")
	}
	bin := filepath.Join(gopath, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", bin, err)
	}
	return sh.RunV("cp", "-v", binaryName, filepath.Join(bin, binaryName))
}
