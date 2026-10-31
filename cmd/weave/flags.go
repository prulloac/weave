package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/compression"
)

type flags struct {
	json         bool
	noCache      bool
	help         bool
	limit        *int
	topK         *int
	mode         contractv1.SearchMode
	types        []string
	tags         []string
	status       string
	failOn       contractv1.AuditFailOn
	verifyKeys   bool
	decrypt      bool
	depth        *int
	direction    contractv1.RelateDirection
	id           string
	db           string
	noDB         bool
	strict       bool
	permissive   bool
	codec        string
	enricher     string
	rerank       string
	timeout      time.Duration
	timeoutSet   bool
	sources      []string
	watch        bool
	events       bool
	out          string
	format       string
	keyID        string
	force        bool
	yes          bool
	provider     string
	dryRun       bool
	noInput      bool
	cacheDir     string
	configPath   string
	noConfig     bool
	strictConfig bool
	explicit     map[string]bool
	positionals  []string
}

func parseFlags(args []string) (flags, error) {
	result := flags{explicit: map[string]bool{}}
	args = expandFlagEquals(args)
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if strings.HasPrefix(arg, "--") {
			result.explicit[arg] = true
		}
		switch arg {
		case "--json":
			result.json = true
		case "--no-cache":
			result.noCache = true
		case "--help", "-h":
			result.help = true
		case "--limit":
			value, next, err := flagValue(args, index, "--limit")
			if err != nil {
				return result, err
			}
			index = next
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 0 {
				return result, fmt.Errorf("--limit requires a non-negative number")
			}
			result.limit = &parsed
		case "--type":
			value, next, err := flagValue(args, index, "--type")
			if err != nil {
				return result, err
			}
			index = next
			result.types = append(result.types, value)
		case "--tag":
			value, next, err := flagValue(args, index, "--tag")
			if err != nil {
				return result, err
			}
			index = next
			result.tags = append(result.tags, value)
		case "--status":
			value, next, err := flagValue(args, index, "--status")
			if err != nil {
				return result, err
			}
			index = next
			result.status = value
		case "--mode":
			value, next, err := flagValue(args, index, "--mode")
			if err != nil {
				return result, err
			}
			index = next
			mode := contractv1.SearchMode(value)
			switch mode {
			case contractv1.SearchKeyword, contractv1.SearchFuzzy, contractv1.SearchHybrid, contractv1.SearchBM25:
				result.mode = mode
			default:
				return result, fmt.Errorf("--mode must be hybrid, keyword, fuzzy or bm25")
			}
		case "--top-k":
			value, next, err := flagValue(args, index, "--top-k")
			if err != nil {
				return result, err
			}
			index = next
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 1 || parsed > 100 {
				return result, fmt.Errorf("--top-k must be between 1 and 100")
			}
			result.topK = &parsed
		case "--fail-on":
			value, next, err := flagValue(args, index, "--fail-on")
			if err != nil {
				return result, err
			}
			index = next
			failOn := contractv1.AuditFailOn(value)
			switch failOn {
			case contractv1.AuditFailOnError, contractv1.AuditFailOnWarning, contractv1.AuditFailOnNone:
				result.failOn = failOn
			default:
				return result, fmt.Errorf("--fail-on must be error, warning or none")
			}
		case "--verify-keys":
			result.verifyKeys = true
		case "--decrypt":
			result.decrypt = true
		case "--id":
			value, next, err := flagValue(args, index, "--id")
			if err != nil {
				return result, err
			}
			index = next
			result.id = value
		case "--db":
			value, next, err := flagValue(args, index, "--db")
			if err != nil {
				return result, err
			}
			index = next
			result.db = value
		case "--no-db":
			result.noDB = true
		case "--strict":
			result.strict = true
		case "--permissive":
			result.permissive = true
		case "--codec":
			value, next, err := flagValue(args, index, "--codec")
			if err != nil {
				return result, err
			}
			index = next
			if !compression.Valid(value) {
				return result, fmt.Errorf("--codec must be zstd, lz4 or none")
			}
			result.codec = value
		case "--with-enricher":
			value, next, err := flagValue(args, index, "--with-enricher")
			if err != nil {
				return result, err
			}
			index = next
			result.enricher = value
		case "--rerank":
			value, next, err := flagValue(args, index, "--rerank")
			if err != nil {
				return result, err
			}
			index = next
			result.rerank = value
		case "--timeout":
			value, next, err := flagValue(args, index, "--timeout")
			if err != nil {
				return result, err
			}
			index = next
			duration, err := time.ParseDuration(value)
			if err != nil || duration <= 0 {
				return result, fmt.Errorf("--timeout requires a positive duration")
			}
			result.timeout = duration
			result.timeoutSet = true
		case "--source":
			value, next, err := flagValue(args, index, "--source")
			if err != nil {
				return result, err
			}
			index = next
			result.sources = append(result.sources, value)
		case "--watch":
			result.watch = true
		case "--events":
			result.events = true
		case "--out":
			value, next, err := flagValue(args, index, "--out")
			if err != nil {
				return result, err
			}
			index = next
			result.out = value
		case "--format":
			value, next, err := flagValue(args, index, "--format")
			if err != nil {
				return result, err
			}
			index = next
			result.format = value
		case "--key-id":
			value, next, err := flagValue(args, index, "--key-id")
			if err != nil {
				return result, err
			}
			index = next
			result.keyID = value
		case "--force":
			result.force = true
		case "--yes":
			result.yes = true
		case "--provider":
			value, next, err := flagValue(args, index, "--provider")
			if err != nil {
				return result, err
			}
			index = next
			result.provider = value
		case "--dry-run":
			result.dryRun = true
		case "--no-input":
			result.noInput = true
		case "--config":
			value, next, err := flagValue(args, index, "--config")
			if err != nil {
				return result, err
			}
			index = next
			result.configPath = value
		case "--no-config":
			result.noConfig = true
		case "--strict-config":
			result.strictConfig = true
		case "--depth":
			value, next, err := flagValue(args, index, "--depth")
			if err != nil {
				return result, err
			}
			index = next
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 0 || parsed > 5 {
				return result, fmt.Errorf("--depth must be between 0 and 5")
			}
			result.depth = &parsed
		case "--direction":
			value, next, err := flagValue(args, index, "--direction")
			if err != nil {
				return result, err
			}
			index = next
			direction := contractv1.RelateDirection(value)
			switch direction {
			case contractv1.RelateOut, contractv1.RelateIn, contractv1.RelateBoth:
				result.direction = direction
			default:
				return result, fmt.Errorf("--direction must be out, in or both")
			}
		case "--":
			result.positionals = append(result.positionals, args[index+1:]...)
			return result, nil
		default:
			if strings.HasPrefix(arg, "-") {
				return result, fmt.Errorf("unknown option: %s", arg)
			}
			result.positionals = append(result.positionals, arg)
		}
	}

	if result.db != "" && result.noDB {
		return result, fmt.Errorf("--db and --no-db are mutually exclusive")
	}
	if result.strict && result.permissive {
		return result, fmt.Errorf("--strict and --permissive are mutually exclusive")
	}
	if result.timeoutSet && result.enricher == "" && result.rerank == "" {
		return result, fmt.Errorf("--timeout requires --with-enricher or --rerank")
	}
	return result, nil
}

func expandFlagEquals(args []string) []string {
	expanded := make([]string, 0, len(args))
	for _, arg := range args {
		if strings.HasPrefix(arg, "--") && strings.Contains(arg, "=") {
			parts := strings.SplitN(arg, "=", 2)
			if parts[0] != "--" && parts[1] != "" {
				expanded = append(expanded, parts[0], parts[1])
				continue
			}
		}
		expanded = append(expanded, arg)
	}
	return expanded
}

func stdinIsTerminal() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func flagValue(args []string, index int, name string) (string, int, error) {
	if index+1 >= len(args) {
		return "", index, fmt.Errorf("%s requires a value", name)
	}
	return args[index+1], index + 1, nil
}
