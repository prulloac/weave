package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
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
	if home, err := os.UserHomeDir(); err == nil {
		db = filepath.Join(home, ".weave", "index.db")
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
	}
}

func defaultConfigPath() string {
	if home := os.Getenv("XDG_CONFIG_HOME"); home != "" {
		return filepath.Join(home, "weave", "config.yaml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".weave/config.yaml"
	}
	if appData := os.Getenv("AppData"); appData != "" {
		return filepath.Join(appData, "weave", "config.yaml")
	}
	return filepath.Join(home, ".config", "weave", "config.yaml")
}

func loadAppConfig(path string, noConfig, strict bool) (appConfig, []string, error) {
	cfg := defaultAppConfig()
	for _, key := range configKeys {
		cfg.Sources[key] = "default"
	}
	if strings.TrimSpace(path) == "" {
		path = os.Getenv("WEAVE_CONFIG")
	}
	if strings.TrimSpace(path) == "" {
		path = defaultConfigPath()
	}
	cfg.Path = path
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		cfg.Exists = true
	}

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
			if err := applyConfigValue(&cfg, key, value, "file"); err != nil {
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
		if err := applyConfigValue(&cfg, key, value, "env"); err != nil {
			return cfg, nil, fmt.Errorf("CONFIG_INVALID: %s: %s", name, strings.TrimPrefix(err.Error(), "CONFIG_INVALID: "))
		}
	}
	return cfg, warnings, nil
}

func applyConfigValue(cfg *appConfig, key string, value any, source string) error {
	invalid := func(reason string) error {
		return fmt.Errorf("CONFIG_INVALID: %s %s", key, reason)
	}
	set := func() { cfg.Sources[key] = source }
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
	if len(positionals) != 1 {
		return usageError(stdout, stderr, jsonMode, "config requires show, path or init")
	}
	if configPath != "" && noConfig {
		return usageError(stdout, stderr, jsonMode, "--config and --no-config are mutually exclusive")
	}
	cfg, warnings, err := loadAppConfig(configPath, noConfig, strict)
	if err != nil {
		return usageError(stdout, stderr, jsonMode, err.Error())
	}
	for _, warning := range warnings {
		fmt.Fprintf(stderr, "%s\n", warning)
	}
	switch positionals[0] {
	case "show":
		entries := make([]map[string]any, 0, len(configKeys))
		for _, key := range configKeys {
			entries = append(entries, map[string]any{
				"key":    key,
				"value":  configValue(cfg, key),
				"source": cfg.Sources[key],
			})
		}
		result := map[string]any{"config_path": cfg.Path, "entries": entries}
		if jsonMode {
			_ = writeJSON(stdout, contractv1.SuccessEnvelope{
				Contract: "weave.contract/v1",
				CLI:      "weave.cli/v1",
				OK:       true,
				Result:   result,
			})
			return 0
		}
		fmt.Fprintf(stdout, "config: %s\n", cfg.Path)
		for _, entry := range entries {
			fmt.Fprintf(stdout, "%-14s %-10s %v\n", entry["key"], entry["source"], entry["value"])
		}
		return 0
	case "path":
		result := map[string]any{"path": cfg.Path, "exists": cfg.Exists}
		if jsonMode {
			_ = writeJSON(stdout, contractv1.SuccessEnvelope{
				Contract: "weave.contract/v1",
				CLI:      "weave.cli/v1",
				OK:       true,
				Result:   result,
			})
			return 0
		}
		fmt.Fprintf(stdout, "%s (exists=%t)\n", cfg.Path, cfg.Exists)
		return 0
	case "init":
		if cfg.Exists && !force {
			return usageError(stdout, stderr, jsonMode, "config file exists; pass --force")
		}
		if err := os.MkdirAll(filepath.Dir(cfg.Path), 0o700); err != nil {
			return usageError(stdout, stderr, jsonMode, err.Error())
		}
		if err := os.WriteFile(cfg.Path, []byte(defaultConfigTemplate), 0o600); err != nil {
			return usageError(stdout, stderr, jsonMode, err.Error())
		}
		if jsonMode {
			_ = writeJSON(stdout, contractv1.SuccessEnvelope{
				Contract: "weave.contract/v1",
				CLI:      "weave.cli/v1",
				OK:       true,
				Result:   map[string]any{"path": cfg.Path, "created": true},
			})
			return 0
		}
		fmt.Fprintf(stdout, "wrote %s\n", cfg.Path)
		return 0
	default:
		return usageError(stdout, stderr, jsonMode, "unknown config command: "+positionals[0])
	}
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

# db: ~/.weave/index.db
# no_db: false
# cache: true
# cache_dir: ""
# codec: zstd            # zstd | lz4 | none
# search_mode: hybrid    # hybrid | keyword | fuzzy | bm25
# top_k: 10
# ingest_mode: permissive # strict | permissive
# timeout: 5s
# enricher: ""
# rerank: ""
`
