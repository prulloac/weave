package main

import (
	"fmt"
	"os"
)

const version = "dev"

const contractVersion = "weave.contract/v1"

func main() {
	if len(os.Args) < 2 {
		printHelp()
		return
	}

	switch os.Args[1] {
	case "--version", "-v", "version":
		fmt.Printf("weave %s (contract 1)\n", version)
	case "--help", "-h", "help":
		printHelp()
	default:
		fmt.Fprintf(os.Stderr, "weave: command not implemented yet: %s\n", os.Args[1])
		os.Exit(2)
	}
}

func printHelp() {
	fmt.Printf(`weave — mount OKF bundles locally

usage:
  weave contract [--json]
  weave parse <path> [--json]
  weave graph <path> [--json]
  weave find <path> <query> [--json]
  weave list <path> [--json]
  weave show <path> <id|file> [--json]
  weave backlinks <path> <id|file> [--json]
  weave path <path> <id|file> <id|file> [--json]
  weave version [--json]
  weave mount <path> [--port <port>] [--build-only]
  weave unmount <port>
  weave status [--prune]
  weave --version | --help

contract: %s
`, contractVersion)
}
