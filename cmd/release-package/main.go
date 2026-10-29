// Command release-package builds release archives, checksums and build info.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/prulloac/weave/release"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	flags := flag.NewFlagSet("release-package", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	version := flags.String("version", "", "release tag, with or without leading v")
	goos := flags.String("os", "", "target GOOS")
	goarch := flags.String("arch", "", "target GOARCH")
	binary := flags.String("binary", "", "path to the built binary")
	root := flags.String("root", ".", "repository root containing README.md and LICENSE")
	out := flags.String("out", "dist", "output directory")
	commit := flags.String("commit", os.Getenv("GITHUB_SHA"), "commit SHA for build info")
	checksums := flags.Bool("checksums", false, "write sorted checksums instead of packaging")
	buildinfo := flags.Bool("buildinfo", false, "write build info instead of packaging")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *version == "" {
		fmt.Fprintln(os.Stderr, "release-package: -version is required")
		return 2
	}
	if *checksums && *buildinfo {
		fmt.Fprintln(os.Stderr, "release-package: -checksums and -buildinfo are mutually exclusive")
		return 2
	}

	switch {
	case *checksums:
		path, err := release.WriteChecksums(*out, *version)
		if err != nil {
			fmt.Fprintf(os.Stderr, "release-package: %s\n", err)
			return 1
		}
		fmt.Fprintln(os.Stdout, path)
	case *buildinfo:
		path, err := release.WriteBuildInfo(*out, *version, *commit, runtime.Version())
		if err != nil {
			fmt.Fprintf(os.Stderr, "release-package: %s\n", err)
			return 1
		}
		fmt.Fprintln(os.Stdout, path)
	default:
		if *goos == "" || *goarch == "" || *binary == "" {
			fmt.Fprintln(os.Stderr, "release-package: -os, -arch and -binary are required")
			return 2
		}
		path, err := release.Package(release.PackageOptions{
			Version: *version,
			GOOS:    *goos,
			GOARCH:  *goarch,
			Binary:  *binary,
			Root:    *root,
			Out:     *out,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "release-package: %s\n", err)
			return 1
		}
		fmt.Fprintln(os.Stdout, path)
	}
	return 0
}
