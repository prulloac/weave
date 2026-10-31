package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine"
	"github.com/prulloac/weave/engine/operations"
	"github.com/prulloac/weave/mcp"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
	"github.com/prulloac/weave/watch"
)

type mcpCLIOptions struct {
	bundle         string
	db             string
	stdio          bool
	listen         string
	listenSet      bool
	token          string
	topKDefault    int
	allowSensitive bool
	watch          bool
	sources        []string
	help           bool
}

func parseMCPOptions(args []string) (mcpCLIOptions, error) {
	options := mcpCLIOptions{listen: "127.0.0.1:7331", topKDefault: 10}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--help", "-h":
			options.help = true
		case "--stdio":
			options.stdio = true
		case "--bundle":
			value, next, err := mcpFlagValue(args, index, "--bundle")
			if err != nil {
				return options, err
			}
			options.bundle = value
			index = next
		case "--db":
			value, next, err := mcpFlagValue(args, index, "--db")
			if err != nil {
				return options, err
			}
			options.db = value
			index = next
		case "--listen":
			value, next, err := mcpFlagValue(args, index, "--listen")
			if err != nil {
				return options, err
			}
			options.listen = value
			options.listenSet = true
			index = next
		case "--token":
			value, next, err := mcpFlagValue(args, index, "--token")
			if err != nil {
				return options, err
			}
			options.token = value
			index = next
		case "--top-k-default":
			value, next, err := mcpFlagValue(args, index, "--top-k-default")
			if err != nil {
				return options, err
			}
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 1 || parsed > 100 {
				return options, fmt.Errorf("--top-k-default must be between 1 and 100")
			}
			options.topKDefault = parsed
			index = next
		case "--allow-sensitive":
			options.allowSensitive = true
		case "--watch":
			options.watch = true
		case "--source":
			value, next, err := mcpFlagValue(args, index, "--source")
			if err != nil {
				return options, err
			}
			options.sources = append(options.sources, value)
			index = next
		default:
			return options, fmt.Errorf("unknown option: %s", arg)
		}
	}
	if options.help {
		return options, nil
	}
	if options.bundle == "" {
		return options, fmt.Errorf("--bundle is required")
	}
	if options.stdio && options.listenSet {
		return options, fmt.Errorf("--stdio and --listen are mutually exclusive")
	}
	if options.watch && options.db == "" {
		return options, fmt.Errorf("--watch requires --db")
	}
	if !options.stdio && !isLoopbackListen(options.listen) && options.token == "" {
		return options, fmt.Errorf("--token is required for non-loopback --listen")
	}
	return options, nil
}

func mcpFlagValue(args []string, index int, name string) (string, int, error) {
	if index+1 >= len(args) {
		return "", index, fmt.Errorf("%s requires a value", name)
	}
	return args[index+1], index + 1, nil
}

func isLoopbackListen(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	switch strings.ToLower(host) {
	case "127.0.0.1", "localhost", "::1":
		return true
	default:
		return false
	}
}

func runMCP(args []string, stdout io.Writer, stderr io.Writer, cfg appConfig) int {
	options, err := parseMCPOptions(args)
	if err != nil {
		fmt.Fprintf(stderr, "weave mcp: %s\n", err)
		return 2
	}
	if options.help {
		printMCPHelp(stdout)
		return 0
	}
	if options.db == "" && !cfg.NoDB && cfg.DB != "" && cfg.Sources["db"] != "default" {
		options.db = cfg.DB
	}

	executor := operations.New(version, engine.Options{})
	var sources []contractv1.SourceSpec
	if len(options.sources) > 0 {
		sources, err = buildSourceSpecs(options.bundle, options.sources)
		if err != nil {
			fmt.Fprintf(stderr, "weave mcp: %s\n", err)
			return 2
		}
	}
	server, err := mcp.New(executor, mcp.Options{
		Bundle:         options.bundle,
		Sources:        sources,
		DB:             options.db,
		TopKDefault:    options.topKDefault,
		AllowSensitive: options.allowSensitive,
	})
	if err != nil {
		fmt.Fprintf(stderr, "weave mcp: %s\n", err)
		return 2
	}
	transport := &mcp.Transport{Server: server, Token: options.token, Version: version}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if options.watch {
		dbPath, err := sqlitestore.ExpandPath(options.db)
		if err != nil {
			fmt.Fprintf(stderr, "weave mcp: %s\n", err)
			return 2
		}
		store, err := sqlitestore.OpenExisting(dbPath)
		if err != nil {
			fmt.Fprintf(stderr, "weave mcp: %s\n", err)
			return 1
		}
		defer store.Close()
		ready := make(chan struct{}, 1)
		watchErr := make(chan error, 1)
		watcher, err := watch.New(watch.Options{
			Root:  options.bundle,
			Store: store,
			OnSync: func(contractv1.SyncReport) {
				select {
				case ready <- struct{}{}:
				default:
				}
			},
			OnError: func(err error) {
				fmt.Fprintf(stderr, "weave mcp: watch: %s\n", err)
			},
		})
		if err != nil {
			fmt.Fprintf(stderr, "weave mcp: %s\n", err)
			return 2
		}
		go func() { watchErr <- watcher.Run(ctx) }()
		select {
		case <-ready:
		case err := <-watchErr:
			fmt.Fprintf(stderr, "weave mcp: watch: %s\n", err)
			return 1
		case <-ctx.Done():
			return 0
		}
		go func() {
			if err := <-watchErr; err != nil {
				fmt.Fprintf(stderr, "weave mcp: watch: %s\n", err)
			}
		}()
	}

	if options.stdio {
		if err := transport.ServeStdio(ctx, os.Stdin, stdout); err != nil {
			fmt.Fprintf(stderr, "weave mcp: %s\n", err)
			return 1
		}
		return 0
	}

	listener, err := net.Listen("tcp", options.listen)
	if err != nil {
		fmt.Fprintf(stderr, "weave mcp: %s\n", err)
		return 1
	}
	defer listener.Close()
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	fmt.Fprintf(stderr, "weave mcp: listening on %s\n", listener.Addr().String())
	if err := transport.Serve(ctx, listener); err != nil {
		fmt.Fprintf(stderr, "weave mcp: %s\n", err)
		return 1
	}
	return 0
}

func printMCPHelp(writer io.Writer) {
	fmt.Fprint(writer, `weave mcp — serve Weave over the Model Context Protocol

usage:
  weave mcp --bundle <path> [--source <alias=path> ...] [--db <file>] [--top-k-default <n>] [--allow-sensitive]
  weave mcp --bundle <path> [--source ...] --stdio [--token <token>]
  weave mcp --bundle <path> [--source ...] --listen 127.0.0.1:7331 [--token <token>]

flags:
  --bundle <path>       primary bundle root (required)
  --source <alias=path> additional bundle source; repeatable
  --db <file>           persistent database path
  --stdio               newline-delimited JSON-RPC over stdin/stdout
  --listen <addr>       TCP listen address (default 127.0.0.1:7331)
  --token <token>       required for non-loopback binds
  --top-k-default <n>   default top-K (default 10)
  --allow-sensitive     expose encrypted/tokenized fields
  --watch               poll and sync the bundle; requires --db
`)
}
