package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
	"gopkg.in/yaml.v3"
)

type appConfig struct {
	DB         string
	NoDB       bool
	Cache      bool
	CacheDir   string
	Codec      string
	SearchMode string
	TopK       int
	IngestMode string
	Timeout    time.Duration
	Enricher   string
	Rerank     string
	Sources    map[string]string
	Origins    map[string]string
	Path       string
	Exists     bool
}

var configKeys = []string{
	"db", "no_db", "cache", "cache_dir", "codec", "search_mode", "top_k",
	"ingest_mode", "timeout", "enricher", "rerank",
}

var reservedConfigKeys = []string{"keys", "security", "token", "api_key"}

func defaultAppConfig() appConfig {
	db := ""
	if path, err := sqlitestore.DefaultPath(); err == nil {
		db = path
	}
	return appConfig{
		DB:         db,
		Cache:      true,
		Codec:      "zstd",
		SearchMode: "hybrid",
		TopK:       10,
		IngestMode: "permissive",
		Timeout:    5 * time.Second,
		Sources:    map[string]string{},
		Origins:    map[string]string{},
	}
}

// defaultConfigPath resolves the global config file with the platform config
// directory. Unix honors XDG_CONFIG_HOME only when it is absolute; Windows
// uses %AppData% and macOS uses Application Support through os.UserConfigDir.
func defaultConfigPath() string {
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
			if filepath.IsAbs(dir) {
				return filepath.Join(dir, "weave", "config.yaml")
			}
			if home, err := os.UserHomeDir(); err == nil {
				return filepath.Join(home, ".config", "weave", "config.yaml")
			}
		}
	}
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "weave", "config.yaml")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "weave", "config.yaml")
	}
	return filepath.Join(".weave", "config.yaml")
}

func resolveConfigPath(path string) (string, bool) {
	if strings.TrimSpace(path) == "" {
		path = os.Getenv("WEAVE_CONFIG")
	}
	if strings.TrimSpace(path) == "" {
		path = defaultConfigPath()
	}
	exists := false
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		exists = true
	}
	return path, exists
}

func loadAppConfig(path string, noConfig, strict bool) (appConfig, []string, error) {
	cfg := defaultAppConfig()
	for _, key := range configKeys {
		cfg.Sources[key] = "default"
		cfg.Origins[key] = "default"
	}
	cfg.Path, cfg.Exists = resolveConfigPath(path)

	warnings := make([]string, 0)
	if !noConfig && cfg.Exists {
		data, err := os.ReadFile(path)
		if err != nil {
			return cfg, nil, fmt.Errorf("CONFIG_INVALID: cannot read %s", path)
		}
		var values map[string]any
		if err := yaml.Unmarshal(data, &values); err != nil {
			return cfg, nil, fmt.Errorf("CONFIG_INVALID: %s", err)
		}
		known := make(map[string]bool, len(configKeys))
		for _, key := range configKeys {
			known[key] = true
		}
		reserved := make(map[string]bool, len(reservedConfigKeys))
		for _, key := range reservedConfigKeys {
			reserved[key] = true
		}
		for key, value := range values {
			if reserved[key] {
				return cfg, nil, fmt.Errorf("CONFIG_INVALID: reserved key %q", key)
			}
			if !known[key] {
				message := "CONFIG_UNKNOWN_KEY: " + key
				if strict {
					return cfg, nil, fmt.Errorf("%s", message)
				}
				warnings = append(warnings, message)
				continue
			}
			if err := applyConfigValue(&cfg, key, value, "file", "file "+path); err != nil {
				return cfg, nil, err
			}
		}
	}

	for key, name := range map[string]string{
		"db":          "WEAVE_DB",
		"no_db":       "WEAVE_NO_DB",
		"cache":       "WEAVE_CACHE",
		"cache_dir":   "WEAVE_CACHE_DIR",
		"codec":       "WEAVE_CODEC",
		"search_mode": "WEAVE_SEARCH_MODE",
		"top_k":       "WEAVE_TOP_K",
		"ingest_mode": "WEAVE_INGEST_MODE",
		"timeout":     "WEAVE_TIMEOUT",
		"enricher":    "WEAVE_ENRICHER",
		"rerank":      "WEAVE_RERANK",
	} {
		value, ok := os.LookupEnv(name)
		if !ok {
			continue
		}
		if key == "enricher" || key == "rerank" {
			if strings.TrimSpace(value) == "" {
				continue
			}
		}
		if err := applyConfigValue(&cfg, key, value, "env", "env "+name); err != nil {
			return cfg, nil, fmt.Errorf("CONFIG_INVALID: %s: %s", name, strings.TrimPrefix(err.Error(), "CONFIG_INVALID: "))
		}
	}
	return cfg, warnings, nil
}

func applyConfigValue(cfg *appConfig, key string, value any, source, origin string) error {
	invalid := func(reason string) error {
		return fmt.Errorf("CONFIG_INVALID: %s %s", key, reason)
	}
	set := func() {
		cfg.Sources[key] = source
		cfg.Origins[key] = origin
	}
	switch key {
	case "db":
		text, ok := value.(string)
		if !ok {
			return invalid("must be a path")
		}
		cfg.DB = strings.TrimSpace(text)
		set()
	case "no_db":
		flag, err := configBool(value)
		if err != nil {
			return invalid("must be a boolean")
		}
		cfg.NoDB = flag
		set()
	case "cache":
		flag, err := configBool(value)
		if err != nil {
			return invalid("must be a boolean")
		}
		cfg.Cache = flag
		set()
	case "cache_dir":
		text, ok := value.(string)
		if !ok {
			return invalid("must be a path")
		}
		cfg.CacheDir = text
		set()
	case "codec":
		text, ok := value.(string)
		if !ok || (text != "zstd" && text != "lz4" && text != "none") {
			return invalid("must be zstd, lz4 or none")
		}
		cfg.Codec = text
		set()
	case "search_mode":
		text, ok := value.(string)
		if !ok || (text != "hybrid" && text != "keyword" && text != "fuzzy" && text != "bm25") {
			return invalid("must be hybrid, keyword, fuzzy or bm25")
		}
		cfg.SearchMode = text
		set()
	case "top_k":
		number, err := configInt(value)
		if err != nil || number < 1 || number > 100 {
			return invalid("must be between 1 and 100")
		}
		cfg.TopK = number
		set()
	case "ingest_mode":
		text, ok := value.(string)
		if !ok || (text != "strict" && text != "permissive") {
			return invalid("must be strict or permissive")
		}
		cfg.IngestMode = text
		set()
	case "timeout":
		text, ok := value.(string)
		if !ok {
			return invalid("must be a duration")
		}
		duration, err := time.ParseDuration(text)
		if err != nil || duration <= 0 {
			return invalid("must be a positive duration")
		}
		cfg.Timeout = duration
		set()
	case "enricher":
		text, ok := value.(string)
		if !ok {
			return invalid("must be a string")
		}
		cfg.Enricher = strings.TrimSpace(text)
		set()
	case "rerank":
		text, ok := value.(string)
		if !ok {
			return invalid("must be a string")
		}
		cfg.Rerank = strings.TrimSpace(text)
		set()
	default:
		return fmt.Errorf("CONFIG_INVALID: unknown key %q", key)
	}
	return nil
}

func configBool(value any) (bool, error) {
	switch typed := value.(type) {
	case bool:
		return typed, nil
	case string:
		return strconv.ParseBool(strings.ToLower(strings.TrimSpace(typed)))
	default:
		return false, fmt.Errorf("not a boolean")
	}
}

func configInt(value any) (int, error) {
	switch typed := value.(type) {
	case int:
		return typed, nil
	case int64:
		return int(typed), nil
	case float64:
		return int(typed), nil
	case string:
		return strconv.Atoi(strings.TrimSpace(typed))
	default:
		return 0, fmt.Errorf("not an integer")
	}
}

func applyConfigToFlags(flags *flags, cfg appConfig, command string) {
	if flags.explicit["--db"] {
		// CLI database wins over config no_db.
	} else if cfg.NoDB {
		flags.noDB = true
	} else if cfg.DB != "" {
		flags.db = cfg.DB
	}
	if !flags.explicit["--no-cache"] {
		flags.noCache = !cfg.Cache
	}
	flags.cacheDir = cfg.CacheDir

	writeCommand := command == "ingest" || command == "rebuild" || command == "sync"
	searchCommand := command == "search" || command == "explain"
	if writeCommand {
		if !flags.explicit["--codec"] && cfg.Sources["codec"] != "default" {
			flags.codec = cfg.Codec
		}
		if !flags.explicit["--strict"] && !flags.explicit["--permissive"] && cfg.Sources["ingest_mode"] != "default" {
			flags.strict = cfg.IngestMode == "strict"
			flags.permissive = cfg.IngestMode == "permissive"
		}
		if !flags.explicit["--with-enricher"] {
			flags.enricher = cfg.Enricher
		}
	}
	if searchCommand {
		if !flags.explicit["--mode"] && cfg.Sources["search_mode"] != "default" {
			flags.mode = contractSearchMode(cfg.SearchMode)
		}
		if !flags.explicit["--top-k"] && cfg.Sources["top_k"] != "default" {
			topK := cfg.TopK
			flags.topK = &topK
		}
		if command == "search" && !flags.explicit["--rerank"] {
			flags.rerank = cfg.Rerank
		}
	}
	if !flags.explicit["--timeout"] && cfg.Sources["timeout"] != "default" && (flags.enricher != "" || flags.rerank != "") {
		flags.timeout = cfg.Timeout
		flags.timeoutSet = true
	}
}

func contractSearchMode(value string) contractv1.SearchMode {
	return contractv1.SearchMode(value)
}

func runConfigCommand(args []string, stdout, stderr io.Writer) int {
	args = expandFlagEquals(args)
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help" ||
		contains(args, "--help") || contains(args, "-h")) {
		printConfigHelp(stdout)
		return 0
	}
	jsonMode := contains(args, "--json")
	configPath := ""
	noConfig := false
	strict := false
	force := false
	positionals := make([]string, 0)
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--json":
			jsonMode = true
		case "--config":
			if index+1 >= len(args) {
				return usageError(stdout, stderr, jsonMode, "--config requires a value")
			}
			configPath = args[index+1]
			index++
		case "--no-config":
			noConfig = true
		case "--strict-config":
			strict = true
		case "--force":
			force = true
		default:
			if strings.HasPrefix(args[index], "--") {
				return usageError(stdout, stderr, jsonMode, "unknown option: "+args[index])
			}
			positionals = append(positionals, args[index])
		}
	}
	subcommand := ""
	if len(positionals) > 0 {
		subcommand = positionals[0]
	}
	switch subcommand {
	case "show", "path", "init":
		if len(positionals) != 1 {
			return usageError(stdout, stderr, jsonMode, "config "+subcommand+" takes no arguments")
		}
	case "get":
		if len(positionals) != 2 {
			return usageError(stdout, stderr, jsonMode, "config get requires a key")
		}
	default:
		return usageError(stdout, stderr, jsonMode, "config requires show, get, path or init")
	}
	if configPath != "" && noConfig {
		return usageError(stdout, stderr, jsonMode, "--config and --no-config are mutually exclusive")
	}
	switch subcommand {
	case "path":
		path, exists := resolveConfigPath(configPath)
		result := contractv1.ConfigPath{Path: path, Exists: exists}
		if jsonMode {
			_ = writeJSON(stdout, successEnvelope(result))
			return 0
		}
		fmt.Fprintf(stdout, "%s (exists=%t)\n", path, exists)
		return 0
	case "init":
		path, exists := resolveConfigPath(configPath)
		if exists && !force {
			return usageError(stdout, stderr, jsonMode, "config file exists; pass --force")
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return usageError(stdout, stderr, jsonMode, err.Error())
		}
		if err := os.WriteFile(path, []byte(defaultConfigTemplate), 0o600); err != nil {
			return usageError(stdout, stderr, jsonMode, err.Error())
		}
		const nextCommand = "weave config show"
		result := contractv1.ConfigInit{Path: path, Created: true, NextCommand: nextCommand}
		if jsonMode {
			_ = writeJSON(stdout, successEnvelope(result))
			return 0
		}
		fmt.Fprintf(stdout, "wrote %s\nnext: %s\n", path, nextCommand)
		return 0
	}

	cfg, warnings, err := loadAppConfig(configPath, noConfig, strict)
	if err != nil {
		return usageError(stdout, stderr, jsonMode, err.Error())
	}
	for _, warning := range warnings {
		fmt.Fprintf(stderr, "%s\n", warning)
	}
	switch subcommand {
	case "show":
		result := configShowResult(cfg)
		if jsonMode {
			_ = writeJSON(stdout, successEnvelope(result))
			return 0
		}
		fmt.Fprintf(stdout, "config: %s\n", cfg.Path)
		for _, entry := range result.Entries {
			display := fmt.Sprintf("%v", entry.Value)
			if text, ok := entry.Value.(string); ok && text == "" {
				display = `""`
			}
			fmt.Fprintf(stdout, "%-14s %-8s %s", entry.Key, entry.Source, display)
			if entry.Origin != "" && entry.Origin != entry.Source {
				fmt.Fprintf(stdout, "  (%s)", entry.Origin)
			}
			fmt.Fprintln(stdout)
		}
		return 0
	case "get":
		key := positionals[1]
		if !contains(configKeys, key) {
			return usageError(stdout, stderr, jsonMode,
				fmt.Sprintf("unknown config key %q; known keys: %s", key, strings.Join(configKeys, ", ")))
		}
		result := contractv1.ConfigGet{
			Key:    key,
			Value:  configValue(cfg, key),
			Source: cfg.Sources[key],
			Origin: cfg.Origins[key],
		}
		if jsonMode {
			_ = writeJSON(stdout, successEnvelope(result))
			return 0
		}
		display := fmt.Sprintf("%v", result.Value)
		if text, ok := result.Value.(string); ok && text == "" {
			display = `""`
		}
		provenance := result.Source
		if result.Origin != "" && result.Origin != result.Source {
			provenance = result.Origin
		}
		fmt.Fprintf(stdout, "%s: %s (%s)\n", key, display, provenance)
		return 0
	default:
		return usageError(stdout, stderr, jsonMode, "config requires show, get, path or init")
	}
}

func configShowResult(cfg appConfig) contractv1.ConfigShow {
	entries := make([]contractv1.ConfigEntry, 0, len(configKeys))
	for _, key := range configKeys {
		entries = append(entries, contractv1.ConfigEntry{
			Key:    key,
			Value:  configValue(cfg, key),
			Source: cfg.Sources[key],
			Origin: cfg.Origins[key],
		})
	}
	return contractv1.ConfigShow{ConfigPath: cfg.Path, Exists: cfg.Exists, Entries: entries}
}

func scanConfigArgs(args []string) (string, bool, bool, error) {
	path := ""
	noConfig := false
	strict := false
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--config":
			if index+1 >= len(args) {
				return "", false, false, fmt.Errorf("--config requires a value")
			}
			path = args[index+1]
			index++
		case "--no-config":
			noConfig = true
		case "--strict-config":
			strict = true
		}
	}
	if path != "" && noConfig {
		return "", false, false, fmt.Errorf("--config and --no-config are mutually exclusive")
	}
	return path, noConfig, strict, nil
}

func stripConfigArgs(args []string) []string {
	filtered := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--config":
			index++
		case "--no-config", "--strict-config":
		default:
			filtered = append(filtered, args[index])
		}
	}
	return filtered
}

func configValue(cfg appConfig, key string) any {
	switch key {
	case "db":
		return cfg.DB
	case "no_db":
		return cfg.NoDB
	case "cache":
		return cfg.Cache
	case "cache_dir":
		return cfg.CacheDir
	case "codec":
		return cfg.Codec
	case "search_mode":
		return cfg.SearchMode
	case "top_k":
		return cfg.TopK
	case "ingest_mode":
		return cfg.IngestMode
	case "timeout":
		return cfg.Timeout.String()
	case "enricher":
		return cfg.Enricher
	case "rerank":
		return cfg.Rerank
	}
	return nil
}

const defaultConfigTemplate = `# Weave global configuration
# Values here are defaults; WEAVE_* environment variables and explicit CLI flags win.
# Set WEAVE_CONFIG to use a different file.

# db: ~/.weave/index.db  # v1.0 keeps the legacy path; use --db or WEAVE_DB
# no_db: false
# cache: true
# cache_dir: ""           # empty uses the user cache directory
# codec: zstd            # zstd | lz4 | none
# search_mode: hybrid    # hybrid | keyword | fuzzy | bm25
# top_k: 10
# ingest_mode: permissive # strict | permissive
# timeout: 5s
# enricher: ""
# rerank: ""
`
