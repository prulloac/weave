package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine"
	"github.com/prulloac/weave/engine/operations"
	"github.com/prulloac/weave/internal/lockfile"
	"github.com/prulloac/weave/mcp"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
)

const workspaceManifestName = "weave.workspace.yaml"

type workspaceFile struct {
	Version int                        `yaml:"version"`
	Index   string                     `yaml:"index,omitempty"`
	Sources map[string]workspaceSource `yaml:"sources,omitempty"`
}

// workspaceSource accepts either a local path string or a provider object.
type workspaceSource struct {
	Path       string
	Git        string
	Ref        string
	Worktree   bool
	Submodules bool
	LFS        bool
	History    bool
	HistoryLim int
}

func (s *workspaceSource) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		s.Path = node.Value
		return nil
	}
	var value struct {
		Path       string `yaml:"path"`
		Git        string `yaml:"git"`
		Ref        string `yaml:"ref"`
		Worktree   bool   `yaml:"worktree"`
		Submodules bool   `yaml:"submodules"`
		LFS        bool   `yaml:"lfs"`
		History    bool   `yaml:"history"`
		HistoryLim int    `yaml:"history_limit"`
	}
	if err := node.Decode(&value); err != nil {
		return err
	}
	s.Path = value.Path
	s.Git = value.Git
	s.Ref = value.Ref
	s.Worktree = value.Worktree
	s.Submodules = value.Submodules
	s.LFS = value.LFS
	s.History = value.History
	s.HistoryLim = value.HistoryLim
	return nil
}

func (s workspaceSource) MarshalYAML() (any, error) {
	if s.Git == "" {
		return s.Path, nil
	}
	return struct {
		Git        string `yaml:"git"`
		Ref        string `yaml:"ref,omitempty"`
		Worktree   bool   `yaml:"worktree,omitempty"`
		Submodules bool   `yaml:"submodules,omitempty"`
		LFS        bool   `yaml:"lfs,omitempty"`
		History    bool   `yaml:"history,omitempty"`
		HistoryLim int    `yaml:"history_limit,omitempty"`
	}{Git: s.Git, Ref: s.Ref, Worktree: s.Worktree, Submodules: s.Submodules, LFS: s.LFS, History: s.History, HistoryLim: s.HistoryLim}, nil
}

// workspace is a resolved manifest with absolute paths.
type workspace struct {
	Dir     string
	Index   string
	Sources []contractv1.SourceSpec
}

func isReservedSourceAlias(alias string) bool {
	return alias == "contract" || alias == "weave" || alias == "workspace"
}

func expandTilde(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

// findWorkspaceManifest searches start and its ancestors for the manifest.
func findWorkspaceManifest(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, workspaceManifestName)
		if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("WORKSPACE_NOT_FOUND")
		}
		dir = parent
	}
}

func loadWorkspace(target string) (workspace, error) {
	manifestPath := target
	info, err := os.Stat(target)
	switch {
	case err == nil && info.IsDir():
		manifestPath = filepath.Join(target, workspaceManifestName)
	case err != nil && strings.HasSuffix(target, workspaceManifestName):
		// treat as file even if relative; resolve below
	case err != nil:
		manifestPath, err = findWorkspaceManifest(target)
		if err != nil {
			return workspace{}, err
		}
	}

	manifestPath, err = filepath.Abs(manifestPath)
	if err != nil {
		return workspace{}, err
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return workspace{}, fmt.Errorf("WORKSPACE_NOT_FOUND")
	}

	var file workspaceFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return workspace{}, fmt.Errorf("WORKSPACE_INVALID: %v", err)
	}
	if file.Version != 1 {
		return workspace{}, fmt.Errorf("WORKSPACE_INVALID: unsupported version %d", file.Version)
	}

	dir := filepath.Dir(manifestPath)
	index, err := expandTilde(file.Index)
	if err != nil {
		return workspace{}, err
	}
	if index == "" {
		index = filepath.Join(dir, ".weave", "index.db")
	} else if !filepath.IsAbs(index) {
		index = filepath.Join(dir, index)
	}
	index, _ = filepath.Abs(index)

	sources := make([]contractv1.SourceSpec, 0, len(file.Sources))
	seenPaths := map[string]string{}
	for alias, source := range file.Sources {
		if !validSourceAlias(alias) || isReservedSourceAlias(alias) {
			return workspace{}, fmt.Errorf("WORKSPACE_INVALID: invalid source alias %q", alias)
		}
		spec := contractv1.SourceSpec{Alias: alias}
		var identity string
		if source.Git != "" {
			spec.Path = source.Git
			spec.Provider = "git"
			spec.Ref = source.Ref
			spec.Worktree = source.Worktree
			spec.Submodules = source.Submodules
			spec.LFS = source.LFS
			spec.History = source.History
			spec.HistoryLimit = source.HistoryLim
			identity = strings.TrimSpace(source.Git)
		} else {
			resolved, err := expandTilde(source.Path)
			if err != nil {
				return workspace{}, err
			}
			if !filepath.IsAbs(resolved) {
				resolved = filepath.Join(dir, resolved)
			}
			resolved, _ = filepath.Abs(resolved)
			spec.Path = resolved
			identity = resolved
		}
		if existing, ok := seenPaths[identity]; ok {
			return workspace{}, fmt.Errorf("WORKSPACE_INVALID: aliases %q and %q share source %q", existing, alias, identity)
		}
		seenPaths[identity] = alias
		sources = append(sources, spec)
	}
	sortSourceSpecs(sources)

	return workspace{Dir: dir, Index: index, Sources: sources}, nil
}

func sortSourceSpecs(specs []contractv1.SourceSpec) {
	for i := 1; i < len(specs); i++ {
		for j := i; j > 0 && specs[j].Alias < specs[j-1].Alias; j-- {
			specs[j], specs[j-1] = specs[j-1], specs[j]
		}
	}
}

func writeWorkspaceFile(dir string, file workspaceFile) error {
	if file.Sources == nil {
		file.Sources = map[string]workspaceSource{}
	}
	data, err := yaml.Marshal(file)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, workspaceManifestName), data, 0o644)
}

func loadWorkspaceFile(dir string) (workspaceFile, string, error) {
	path := filepath.Join(dir, workspaceManifestName)
	data, err := os.ReadFile(path)
	if err != nil {
		return workspaceFile{}, path, err
	}
	var file workspaceFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return workspaceFile{}, path, err
	}
	if file.Sources == nil {
		file.Sources = map[string]workspaceSource{}
	}
	return file, path, nil
}

func runWorkspaceCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		printCommandHelp(stdout, "workspace")
		return 0
	}
	if len(args) == 0 {
		fmt.Fprintln(stderr, "workspace requires a subcommand: init, show or path")
		return 2
	}
	subcommand := args[0]
	rest := args[1:]

	workspaceArg := ""
	jsonOut := false
	positionals := make([]string, 0)
	for index := 0; index < len(rest); index++ {
		arg := rest[index]
		switch arg {
		case "--json":
			jsonOut = true
		case "--workspace":
			if index+1 >= len(rest) {
				fmt.Fprintln(stderr, "--workspace requires a value")
				return 2
			}
			workspaceArg = rest[index+1]
			index++
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(stderr, "unknown option: %s\n", arg)
				return 2
			}
			positionals = append(positionals, arg)
		}
	}

	switch subcommand {
	case "init":
		dir := "."
		if len(positionals) > 0 {
			dir = positionals[0]
		}
		if len(positionals) > 1 {
			fmt.Fprintln(stderr, "workspace init accepts at most one directory")
			return 2
		}
		dir, err := filepath.Abs(dir)
		if err != nil {
			fmt.Fprintf(stderr, "weave: %v\n", err)
			return 2
		}
		path := filepath.Join(dir, workspaceManifestName)
		if _, err := os.Stat(path); err == nil {
			fmt.Fprintf(stderr, "weave: workspace already exists: %s\n", path)
			return 1
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(stderr, "weave: %v\n", err)
			return 1
		}
		if err := writeWorkspaceFile(dir, workspaceFile{Version: 1}); err != nil {
			fmt.Fprintf(stderr, "weave: %v\n", err)
			return 1
		}
		if jsonOut {
			fmt.Fprintf(stdout, `{"path":%q,"created":true}`+"\n", path)
		} else {
			fmt.Fprintf(stdout, "wrote %s\n", path)
		}
		return 0
	case "path":
		ws, err := loadWorkspace(firstNonEmpty(workspaceArg, "."))
		if err != nil {
			fmt.Fprintf(stderr, "weave: %v\n", err)
			return 2
		}
		if jsonOut {
			fmt.Fprintf(stdout, `{"path":%q}`+"\n", filepath.Join(ws.Dir, workspaceManifestName))
		} else {
			fmt.Fprintln(stdout, filepath.Join(ws.Dir, workspaceManifestName))
		}
		return 0
	case "show":
		ws, err := loadWorkspace(firstNonEmpty(workspaceArg, "."))
		if err != nil {
			fmt.Fprintf(stderr, "weave: %v\n", err)
			return 2
		}
		if jsonOut {
			sources := append([]contractv1.SourceSpec(nil), ws.Sources...)
			if err := writeJSON(stdout, contractv1.SuccessEnvelope{
				Contract: "weave.contract/v1", CLI: "weave.cli/v1", OK: true,
				Result: map[string]any{"dir": ws.Dir, "index": ws.Index, "sources": sources},
			}); err != nil {
				fmt.Fprintf(stderr, "weave: %v\n", err)
				return 1
			}
			return 0
		}
		fmt.Fprintf(stdout, "workspace: %s\n", ws.Dir)
		fmt.Fprintf(stdout, "index: %s\n", ws.Index)
		for _, source := range ws.Sources {
			fmt.Fprintf(stdout, "source %s: %s\n", source.Alias, source.Path)
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown workspace subcommand: %s\n", subcommand)
		return 2
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func runSourceCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		printCommandHelp(stdout, "source")
		return 0
	}
	if len(args) == 0 {
		fmt.Fprintln(stderr, "source requires a subcommand: add, remove, list or show")
		return 2
	}
	subcommand := args[0]
	rest := args[1:]

	workspaceArg := ""
	jsonOut := false
	prune := false
	gitSource := false
	gitRef := ""
	gitWorktree := false
	gitSubmodules := false
	gitLFS := false
	gitHistory := false
	gitHistoryLim := 0
	positionals := make([]string, 0)
	for index := 0; index < len(rest); index++ {
		arg := rest[index]
		switch arg {
		case "--json":
			jsonOut = true
		case "--prune":
			prune = true
		case "--git":
			gitSource = true
		case "--ref":
			if index+1 >= len(rest) {
				fmt.Fprintln(stderr, "--ref requires a value")
				return 2
			}
			gitRef = rest[index+1]
			index++
		case "--worktree":
			gitWorktree = true
		case "--submodules":
			gitSubmodules = true
		case "--lfs":
			gitLFS = true
		case "--history":
			gitHistory = true
		case "--history-limit":
			if index+1 >= len(rest) {
				fmt.Fprintln(stderr, "--history-limit requires a value")
				return 2
			}
			limit, err := strconv.Atoi(rest[index+1])
			if err != nil || limit < 1 || limit > 100 {
				fmt.Fprintln(stderr, "--history-limit requires an integer between 1 and 100")
				return 2
			}
			gitHistoryLim = limit
			index++
		case "--workspace":
			if index+1 >= len(rest) {
				fmt.Fprintln(stderr, "--workspace requires a value")
				return 2
			}
			workspaceArg = rest[index+1]
			index++
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(stderr, "unknown option: %s\n", arg)
				return 2
			}
			positionals = append(positionals, arg)
		}
	}

	switch subcommand {
	case "add":
		if len(positionals) != 2 {
			fmt.Fprintln(stderr, "source add requires <alias> <path>")
			return 2
		}
		return addSource(positionals[0], positionals[1], firstNonEmpty(workspaceArg, "."), gitSource, gitRef, gitWorktree, gitSubmodules, gitLFS, gitHistory, gitHistoryLim, jsonOut, stdout, stderr)
	case "remove":
		if len(positionals) != 1 {
			fmt.Fprintln(stderr, "source remove requires <alias>")
			return 2
		}
		return removeSource(positionals[0], firstNonEmpty(workspaceArg, "."), prune, jsonOut, stdout, stderr)
	case "list":
		return listSources(firstNonEmpty(workspaceArg, "."), jsonOut, stdout, stderr)
	case "show":
		if len(positionals) != 1 {
			fmt.Fprintln(stderr, "source show requires <alias>")
			return 2
		}
		return showSource(positionals[0], firstNonEmpty(workspaceArg, "."), jsonOut, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown source subcommand: %s\n", subcommand)
		return 2
	}
}

func addSource(alias, rawPath, workspaceArg string, gitSource bool, gitRef string, gitWorktree bool, gitSubmodules bool, gitLFS bool, gitHistory bool, gitHistoryLim int, jsonOut bool, stdout, stderr io.Writer) int {
	if !validSourceAlias(alias) || isReservedSourceAlias(alias) {
		fmt.Fprintf(stderr, "weave: INVALID_REQUEST: invalid source alias %q\n", alias)
		return 2
	}
	if gitRef != "" || gitWorktree || gitSubmodules || gitLFS || gitHistory || gitHistoryLim != 0 {
		if !gitSource {
			fmt.Fprintln(stderr, "weave: --ref, --worktree, --submodules, --lfs and --history require --git")
			return 2
		}
	}
	if gitHistoryLim != 0 && !gitHistory {
		fmt.Fprintln(stderr, "weave: --history-limit requires --history")
		return 2
	}
	ws, err := loadWorkspace(workspaceArg)
	if err != nil {
		fmt.Fprintf(stderr, "weave: %v\n", err)
		return 2
	}
	file, _, err := loadWorkspaceFile(ws.Dir)
	if err != nil {
		fmt.Fprintf(stderr, "weave: %v\n", err)
		return 2
	}
	if _, exists := file.Sources[alias]; exists {
		fmt.Fprintf(stderr, "weave: WORKSPACE_INVALID: duplicate source alias %q\n", alias)
		return 2
	}
	spec := contractv1.SourceSpec{Alias: alias, Path: rawPath}
	var source workspaceSource
	identity := ""
	if gitSource {
		rawPath = strings.TrimSpace(rawPath)
		if rawPath == "" {
			fmt.Fprintln(stderr, "weave: source URI is required")
			return 2
		}
		spec = contractv1.SourceSpec{Alias: alias, Path: rawPath, Provider: "git", Ref: gitRef, Worktree: gitWorktree, Submodules: gitSubmodules, LFS: gitLFS, History: gitHistory, HistoryLimit: gitHistoryLim}
		source = workspaceSource{Git: rawPath, Ref: gitRef, Worktree: gitWorktree, Submodules: gitSubmodules, LFS: gitLFS, History: gitHistory, HistoryLim: gitHistoryLim}
		identity = rawPath
	} else {
		resolved, err := resolveSourcePath(ws.Dir, rawPath)
		if err != nil {
			fmt.Fprintf(stderr, "weave: %v\n", err)
			return 2
		}
		spec.Path = resolved
		source = workspaceSource{Path: rawPath}
		identity = resolved
	}
	for otherAlias, other := range file.Sources {
		otherIdentity := other.Git
		if otherIdentity == "" {
			otherResolved, _ := resolveSourcePath(ws.Dir, other.Path)
			otherIdentity = otherResolved
		}
		if otherIdentity == identity {
			fmt.Fprintf(stderr, "weave: WORKSPACE_INVALID: aliases %q and %q share source %q\n", otherAlias, alias, identity)
			return 2
		}
	}
	file.Sources[alias] = source
	if err := writeWorkspaceFile(ws.Dir, file); err != nil {
		fmt.Fprintf(stderr, "weave: %v\n", err)
		return 1
	}

	if err := indexSource(ws.Index, alias, spec, ws.Dir); err != nil {
		fmt.Fprintf(stderr, "weave: SOURCE_DEGRADED: %s: %v\n", alias, err)
	}

	if jsonOut {
		if err := writeJSON(stdout, contractv1.SuccessEnvelope{
			Contract: "weave.contract/v1", CLI: "weave.cli/v1", OK: true,
			Result: map[string]any{"alias": alias, "path": spec.Path},
		}); err != nil {
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "added source %s -> %s\n", alias, spec.Path)
	return 0
}

func resolveSourcePath(dir, rawPath string) (string, error) {
	expanded, err := expandTilde(rawPath)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(expanded) {
		expanded = filepath.Join(dir, expanded)
	}
	return filepath.Abs(expanded)
}

// acquireWorkspaceLock takes the single-writer lock for an index database.
func acquireWorkspaceLock(index string) (*lockfile.Lock, error) {
	if err := os.MkdirAll(filepath.Dir(index), 0o700); err != nil {
		return nil, err
	}
	return lockfile.Acquire(index + ".lock")
}

// indexSource ingests one source into the workspace index and records its status.
func indexSource(index, alias string, spec contractv1.SourceSpec, workspaceRoot string) error {
	lock, err := acquireWorkspaceLock(index)
	if err != nil {
		return err
	}
	defer lock.Release()

	executor := operations.New(version, engine.Options{})
	envelope := executor.Execute(context.Background(), contractv1.OperationRequest{
		Op:            contractv1.OpIngest,
		Path:          spec.Path,
		Sources:       []contractv1.SourceSpec{spec},
		WorkspaceRoot: workspaceRoot,
		DB:            index,
	})

	store, err := sqlitestore.Open(index)
	if err != nil {
		return err
	}
	defer store.Close()

	status := sqlitestore.SourceActive
	if _, ok := envelope.(contractv1.FailureEnvelope); ok {
		status = sqlitestore.SourceDegraded
	}
	if _, exists, err := store.SourceByAlias(context.Background(), alias); err == nil && exists {
		return store.SetSourceStatus(context.Background(), alias, status)
	}
	return store.UpsertSource(context.Background(), sqlitestore.SourceRecord{
		Alias: alias, Path: spec.Path, Status: status, Generation: 1, Provider: spec.Provider,
	})
}

func removeSource(alias, workspaceArg string, prune bool, jsonOut bool, stdout, stderr io.Writer) int {
	ws, err := loadWorkspace(workspaceArg)
	if err != nil {
		fmt.Fprintf(stderr, "weave: %v\n", err)
		return 2
	}
	file, _, err := loadWorkspaceFile(ws.Dir)
	if err != nil {
		fmt.Fprintf(stderr, "weave: %v\n", err)
		return 2
	}
	if _, exists := file.Sources[alias]; !exists {
		fmt.Fprintf(stderr, "weave: SOURCE_NOT_FOUND: %s\n", alias)
		return 1
	}
	delete(file.Sources, alias)
	if err := writeWorkspaceFile(ws.Dir, file); err != nil {
		fmt.Fprintf(stderr, "weave: %v\n", err)
		return 1
	}

	if store, openErr := sqlitestore.OpenExisting(ws.Index); openErr == nil {
		if prune {
			_ = store.PruneSource(context.Background(), alias)
		} else {
			_ = store.RemoveSource(context.Background(), alias)
		}
		store.Close()
	}

	if jsonOut {
		if err := writeJSON(stdout, contractv1.SuccessEnvelope{
			Contract: "weave.contract/v1", CLI: "weave.cli/v1", OK: true,
			Result: map[string]any{"alias": alias},
		}); err != nil {
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "removed source %s\n", alias)
	return 0
}

func listSources(workspaceArg string, jsonOut bool, stdout, stderr io.Writer) int {
	ws, err := loadWorkspace(workspaceArg)
	if err != nil {
		fmt.Fprintf(stderr, "weave: %v\n", err)
		return 2
	}
	records := []sqlitestore.SourceRecord{}
	if store, openErr := sqlitestore.OpenExisting(ws.Index); openErr == nil {
		records, _ = store.ListSources(context.Background())
		store.Close()
	}

	if jsonOut {
		items := make([]map[string]any, 0, len(records))
		for _, record := range records {
			items = append(items, sourceListItem(record))
		}
		if err := writeJSON(stdout, contractv1.SuccessEnvelope{
			Contract: "weave.contract/v1", CLI: "weave.cli/v1", OK: true,
			Result: map[string]any{"sources": items},
		}); err != nil {
			return 1
		}
		return 0
	}
	for _, record := range records {
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", record.Alias, record.Status, record.Path)
	}
	return 0
}

func sourceListItem(record sqlitestore.SourceRecord) map[string]any {
	return map[string]any{
		"alias":    record.Alias,
		"path":     record.Path,
		"status":   string(record.Status),
		"provider": sourceProvider(record),
		"uri":      sourceURI(record),
		"ref":      sourceMetadata(record, "ref_name"),
		"commit":   sourceMetadata(record, "commit_sha"),
	}
}

func sourceProvider(record sqlitestore.SourceRecord) string {
	if strings.TrimSpace(record.Provider) == "" {
		return "path"
	}
	return record.Provider
}

func sourceURI(record sqlitestore.SourceRecord) string {
	if value := sourceMetadata(record, "uri"); value != "" {
		return value
	}
	return record.Path
}

func sourceMetadata(record sqlitestore.SourceRecord, key string) string {
	if len(record.Metadata) == 0 {
		return ""
	}
	var values map[string]any
	if json.Unmarshal(record.Metadata, &values) != nil {
		return ""
	}
	value, _ := values[key].(string)
	return value
}

func showSource(alias, workspaceArg string, jsonOut bool, stdout, stderr io.Writer) int {
	ws, err := loadWorkspace(workspaceArg)
	if err != nil {
		fmt.Fprintf(stderr, "weave: %v\n", err)
		return 2
	}
	record, ok, err := sourceRecord(ws.Index, alias)
	if err != nil {
		fmt.Fprintf(stderr, "weave: %v\n", err)
		return 1
	}
	if !ok {
		fmt.Fprintf(stderr, "weave: SOURCE_NOT_FOUND: %s\n", alias)
		return 1
	}
	if jsonOut {
		if err := writeJSON(stdout, contractv1.SuccessEnvelope{
			Contract: "weave.contract/v1", CLI: "weave.cli/v1", OK: true,
			Result: sourceListItem(record),
		}); err != nil {
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "%s\t%s\t%s\n", record.Alias, record.Status, record.Path)
	return 0
}

func sourceRecord(index, alias string) (sqlitestore.SourceRecord, bool, error) {
	store, err := sqlitestore.OpenExisting(index)
	if err != nil {
		return sqlitestore.SourceRecord{}, false, nil
	}
	defer store.Close()
	return store.SourceByAlias(context.Background(), alias)
}

// recordWorkspaceReport updates registry statuses from a multi-source write report.
func recordWorkspaceReport(index string, value any) {
	envelope, ok := value.(contractv1.SuccessEnvelope)
	if !ok {
		return
	}
	errorsByAlias := map[string]int{}
	switch report := envelope.Result.(type) {
	case contractv1.IngestReport:
		for _, source := range report.Sources {
			errorsByAlias[source.Source] = source.Errors
		}
	case contractv1.SyncReport:
		for _, source := range report.Sources {
			errorsByAlias[source.Source] = source.Errors
		}
	default:
		return
	}
	store, err := sqlitestore.OpenExisting(index)
	if err != nil {
		return
	}
	defer store.Close()
	for alias, failures := range errorsByAlias {
		status := sqlitestore.SourceActive
		if failures > 0 {
			status = sqlitestore.SourceDegraded
		}
		_ = store.SetSourceStatus(context.Background(), alias, status)
	}
}

func sourceStatusMap(index string) map[string]string {
	statuses := map[string]string{}
	store, err := sqlitestore.OpenExisting(index)
	if err != nil {
		return statuses
	}
	defer store.Close()
	records, err := store.ListSources(context.Background())
	if err != nil {
		return statuses
	}
	for _, record := range records {
		statuses[record.Alias] = string(record.Status)
	}
	return statuses
}

// resolvedWorkspaceSources maps manifest sources to their indexed snapshot paths and provenance.
func resolvedWorkspaceSources(ws workspace) ([]contractv1.SourceSpec, map[string]mcp.SourceProvenance) {
	specs := append([]contractv1.SourceSpec(nil), ws.Sources...)
	provenance := make(map[string]mcp.SourceProvenance, len(specs))
	records := map[string]sqlitestore.SourceRecord{}
	if store, err := sqlitestore.OpenExisting(ws.Index); err == nil {
		if values, listErr := store.ListSources(context.Background()); listErr == nil {
			for _, record := range values {
				records[record.Alias] = record
			}
		}
		store.Close()
	}
	for index := range specs {
		spec := &specs[index]
		provider := spec.Provider
		if provider == "" {
			provider = "path"
		}
		value := mcp.SourceProvenance{Provider: provider, URI: spec.Path}
		if record, ok := records[spec.Alias]; ok {
			if record.Path != "" {
				spec.Path = record.Path
			}
			value.Provider = sourceProvider(record)
			value.URI = sourceURI(record)
			value.Ref = sourceMetadata(record, "ref_name")
			value.Commit = sourceMetadata(record, "commit_sha")
			value.Host = sourceMetadata(record, "host")
			value.Owner = sourceMetadata(record, "owner")
			value.Name = sourceMetadata(record, "name")
		}
		if value.Provider == "" {
			value.Provider = "path"
		}
		if value.URI == "" {
			value.URI = spec.Path
		}
		provenance[spec.Alias] = value
	}
	return specs, provenance
}

// reconcileOnce indexes new sources and swaps the served source set.
func reconcileOnce(workspaceArg string, server *mcp.Server, stderr io.Writer) error {
	ws, err := loadWorkspace(workspaceArg)
	if err != nil {
		return err
	}
	for _, source := range ws.Sources {
		record, ok, _ := sourceRecord(ws.Index, source.Alias)
		if !ok || record.Status == sqlitestore.SourceIndexing {
			if err := indexSource(ws.Index, source.Alias, source, ws.Dir); err != nil {
				fmt.Fprintf(stderr, "weave mcp: reconcile %s: %v\n", source.Alias, err)
			}
		}
	}
	sources, provenance := resolvedWorkspaceSources(ws)
	if err := server.SetSources(sources, sourceStatusMap(ws.Index)); err != nil {
		return err
	}
	server.SetSourceProvenance(provenance)
	return nil
}

// reconcileWorkspace watches the manifest and re-reconciles sources at runtime.
func reconcileWorkspace(ctx context.Context, workspaceArg, index string, server *mcp.Server, stderr io.Writer) {
	lastHash := ""
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ws, err := loadWorkspace(workspaceArg)
			if err != nil {
				continue
			}
			data, err := os.ReadFile(filepath.Join(ws.Dir, workspaceManifestName))
			if err != nil {
				continue
			}
			hash := fmt.Sprintf("%x", sha256.Sum256(data))
			if hash == lastHash {
				continue
			}
			lastHash = hash
			if err := reconcileOnce(workspaceArg, server, stderr); err != nil {
				fmt.Fprintf(stderr, "weave mcp: reconcile: %v\n", err)
			}
		}
	}
}
