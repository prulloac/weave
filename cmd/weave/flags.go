package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/codecs"
)

type flags struct {
	json           bool
	quiet          bool
	verbose        bool
	ascii          bool
	short          bool
	noCache        bool
	help           bool
	limit          *int
	topK           *int
	mode           contractv1.SearchMode
	types          []string
	tags           []string
	status         string
	failOn         contractv1.AuditFailOn
	verifyKeys     bool
	decrypt        bool
	depth          *int
	direction      contractv1.RelateDirection
	relations      []string
	id             string
	extensionName  string
	docID          string
	db             string
	workspace      string
	noDB           bool
	strict         bool
	permissive     bool
	assets         bool
	noAssets       bool
	assetTypes     []string
	codec          string
	enricher       string
	withEmbedder   string
	embedder       string
	rerank         string
	scorer         string
	scorerWeight   *float64
	candidateK     *int
	timeout        time.Duration
	timeoutSet     bool
	sources        []string
	git            bool
	gitRef         string
	gitWorktree    bool
	gitSubmodules  bool
	gitLFS         bool
	gitHistory     bool
	gitHistoryLim  int
	providerOption []string
	watch          bool
	events         bool
	out            string
	format         string
	keyID          string
	force          bool
	yes            bool
	provider       string
	dryRun         bool
	noInput        bool
	cacheDir       string
	configPath     string
	noConfig       bool
	strictConfig   bool
	model          string
	dim            int
	keepSource     bool
	explicit       map[string]bool
	positionals    []string
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
		case "--quiet":
			result.quiet = true
		case "--verbose":
			result.verbose = true
		case "--ascii":
			result.ascii = true
		case "--short":
			result.short = true
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
			case contractv1.SearchKeyword, contractv1.SearchFuzzy, contractv1.SearchHybrid,
				contractv1.SearchBM25, contractv1.SearchSemantic:
				result.mode = mode
			default:
				return result, fmt.Errorf("--mode must be hybrid, keyword, fuzzy, bm25 or semantic")
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
		case "--extension":
			value, next, err := flagValue(args, index, "--extension")
			if err != nil {
				return result, err
			}
			index = next
			result.extensionName = value
		case "--doc":
			value, next, err := flagValue(args, index, "--doc")
			if err != nil {
				return result, err
			}
			index = next
			result.docID = value
		case "--db":
			value, next, err := flagValue(args, index, "--db")
			if err != nil {
				return result, err
			}
			index = next
			result.db = value
		case "--workspace":
			value, next, err := flagValue(args, index, "--workspace")
			if err != nil {
				return result, err
			}
			index = next
			result.workspace = value
		case "--no-db":
			result.noDB = true
		case "--strict":
			result.strict = true
		case "--permissive":
			result.permissive = true
		case "--assets":
			result.assets = true
		case "--no-assets":
			result.noAssets = true
		case "--asset-types":
			value, next, err := flagValue(args, index, "--asset-types")
			if err != nil {
				return result, err
			}
			index = next
			for _, item := range strings.Split(value, ",") {
				item = strings.ToLower(strings.TrimSpace(item))
				if item != "" {
					result.assetTypes = append(result.assetTypes, item)
				}
			}
			if len(result.assetTypes) == 0 {
				return result, fmt.Errorf("--asset-types requires at least one MIME type")
			}
		case "--codec":
			value, next, err := flagValue(args, index, "--codec")
			if err != nil {
				return result, err
			}
			index = next
			if !codecs.ValidName(value) {
				return result, fmt.Errorf("--codec must be a valid codec name")
			}
			result.codec = value
		case "--with-enricher":
			value, next, err := flagValue(args, index, "--with-enricher")
			if err != nil {
				return result, err
			}
			index = next
			result.enricher = value
		case "--with-embedder":
			value, next, err := flagValue(args, index, "--with-embedder")
			if err != nil {
				return result, err
			}
			index = next
			result.withEmbedder = value
		case "--embedder":
			value, next, err := flagValue(args, index, "--embedder")
			if err != nil {
				return result, err
			}
			index = next
			result.embedder = value
		case "--rerank":
			value, next, err := flagValue(args, index, "--rerank")
			if err != nil {
				return result, err
			}
			index = next
			result.rerank = value
		case "--scorer":
			value, next, err := flagValue(args, index, "--scorer")
			if err != nil {
				return result, err
			}
			index = next
			result.scorer = value
		case "--scorer-weight":
			value, next, err := flagValue(args, index, "--scorer-weight")
			if err != nil {
				return result, err
			}
			index = next
			parsed, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return result, fmt.Errorf("--scorer-weight requires a number between 0 and 1")
			}
			result.scorerWeight = &parsed
		case "--candidate-k":
			value, next, err := flagValue(args, index, "--candidate-k")
			if err != nil {
				return result, err
			}
			index = next
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return result, fmt.Errorf("--candidate-k requires a number")
			}
			result.candidateK = &parsed
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
		case "--git":
			result.git = true
		case "--ref":
			value, next, err := flagValue(args, index, "--ref")
			if err != nil {
				return result, err
			}
			index = next
			result.gitRef = value
		case "--worktree":
			result.gitWorktree = true
		case "--submodules":
			result.gitSubmodules = true
		case "--lfs":
			result.gitLFS = true
		case "--history":
			result.gitHistory = true
		case "--history-limit":
			value, next, err := flagValue(args, index, "--history-limit")
			if err != nil {
				return result, err
			}
			index = next
			limit, err := strconv.Atoi(value)
			if err != nil || limit < 1 || limit > 100 {
				return result, fmt.Errorf("--history-limit requires an integer between 1 and 100")
			}
			result.gitHistoryLim = limit
		case "--provider-option":
			value, next, err := flagValue(args, index, "--provider-option")
			if err != nil {
				return result, err
			}
			index = next
			if key, _, ok := strings.Cut(value, "="); !ok || strings.TrimSpace(key) == "" {
				return result, fmt.Errorf("--provider-option requires key=value")
			}
			result.providerOption = append(result.providerOption, value)
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
		case "--model":
			value, next, err := flagValue(args, index, "--model")
			if err != nil {
				return result, err
			}
			index = next
			result.model = value
		case "--dim":
			value, next, err := flagValue(args, index, "--dim")
			if err != nil {
				return result, err
			}
			index = next
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed <= 0 {
				return result, fmt.Errorf("--dim requires a positive number")
			}
			result.dim = parsed
		case "--keep-source":
			result.keepSource = true
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
		case "--relation":
			value, next, err := flagValue(args, index, "--relation")
			if err != nil {
				return result, err
			}
			index = next
			for _, relation := range strings.Split(value, ",") {
				relation = strings.TrimSpace(relation)
				if relation == "" {
					return result, fmt.Errorf("--relation requires comma-separated relation kinds")
				}
				result.relations = append(result.relations, relation)
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
	if result.quiet && result.verbose {
		return result, fmt.Errorf("--quiet and --verbose are mutually exclusive")
	}
	if result.timeoutSet && result.enricher == "" && result.rerank == "" && result.scorer == "" &&
		result.withEmbedder == "" && result.embedder == "" {
		return result, fmt.Errorf("--timeout requires --with-enricher, --with-embedder, --embedder, --rerank or --scorer")
	}
	if (result.gitRef != "" || result.gitWorktree || result.gitSubmodules || result.gitLFS ||
		result.gitHistory || result.gitHistoryLim != 0 || len(result.providerOption) > 0) && !result.git {
		return result, fmt.Errorf("--ref, --worktree, --submodules, --lfs, --history and --provider-option require --git")
	}
	if result.gitHistoryLim != 0 && !result.gitHistory {
		return result, fmt.Errorf("--history-limit requires --history")
	}
	return result, nil
}

func providerOptionMap(values []string) map[string]any {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]any, len(values))
	for _, value := range values {
		key, raw, ok := strings.Cut(value, "=")
		if !ok {
			continue
		}
		result[strings.TrimSpace(key)] = raw
	}
	return result
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
