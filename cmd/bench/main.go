// Command bench runs Weave's end-to-end benchmark scenarios.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
	"time"

	"github.com/prulloac/weave/benchmarks/benchutil"
	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/okf"
)

const schemaVersion = "weave.bench/v1"

type report struct {
	Schema     string     `json:"schema"`
	Commit     string     `json:"commit"`
	Go         string     `json:"go"`
	Runner     string     `json:"runner"`
	GOMAXPROCS int        `json:"gomaxprocs"`
	Corpus     corpusInfo `json:"corpus"`
	Results    []result   `json:"results"`
}

type corpusInfo struct {
	Name      string `json:"name"`
	SHA256    string `json:"sha256"`
	Docs      int    `json:"documents"`
	BodyBytes int64  `json:"body_bytes"`
	Edges     int    `json:"edges"`
}

type result struct {
	ID        string    `json:"id"`
	Unit      string    `json:"unit"`
	Aggregate string    `json:"aggregate"`
	Value     float64   `json:"value"`
	Samples   []float64 `json:"samples,omitempty"`
	Budget    *float64  `json:"budget,omitempty"`
	Status    string    `json:"status"`
}

type corpusContext struct {
	Path    string
	Name    string
	Summary benchutil.ProfileSummary
	Bundle  *contractv1.Bundle
}

func main() {
	os.Exit(runBench())
}

func runBench() int {
	corpus := flag.String("corpus", benchutil.Corpus("10k"), "corpus directory")
	scenario := flag.String("scenario", "all", "comma-separated scenarios: parse, ingest, sync, search, explain, audit, export, space")
	samples := flag.Int("samples", 5, "samples per scenario")
	weaveBinary := flag.String("weave", "", "weave binary for CLI scenarios (default: look in PATH)")
	baselinePath := flag.String("baseline", filepath.Join(benchutil.Root(), "benchmarks", "baseline", "10k.json"), "baseline report for the 10% gate (empty disables)")
	profile := flag.String("profile", "", "write a CPU profile")
	jsonOutput := flag.Bool("json", false, "print the report as JSON")
	markdown := flag.Bool("markdown", false, "print the report as a Markdown table")
	flag.Parse()

	if *samples < 1 {
		fmt.Fprintln(os.Stderr, "bench: -samples must be at least 1")
		return 2
	}
	if *profile != "" {
		file, err := os.Create(*profile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "bench:", err)
			return 1
		}
		if err := pprof.StartCPUProfile(file); err != nil {
			fmt.Fprintln(os.Stderr, "bench:", err)
			return 1
		}
		defer func() {
			pprof.StopCPUProfile()
			_ = file.Close()
		}()
	}

	corpusState, err := loadCorpus(*corpus)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bench:", err)
		return 2
	}
	registry, err := loadRegistry()
	if err != nil {
		fmt.Fprintln(os.Stderr, "bench:", err)
		return 1
	}
	queries, err := benchutil.LoadQueries()
	if err != nil {
		fmt.Fprintln(os.Stderr, "bench:", err)
		return 1
	}
	value := newReport(corpusState)
	names := scenarioNames(*scenario)
	if err := runScenarios(&value, corpusState, registry, queries, names, *samples, *weaveBinary); err != nil {
		fmt.Fprintln(os.Stderr, "bench:", err)
		return 1
	}
	gateFailed := false
	if *baselinePath != "" {
		if _, err := os.Stat(*baselinePath); err == nil {
			if err := compareBaseline(&value, *baselinePath, registry); err != nil {
				fmt.Fprintln(os.Stderr, "bench:", err)
				gateFailed = true
			}
		}
	}
	if *jsonOutput {
		data, err := json.MarshalIndent(value, "", "\t")
		if err != nil {
			fmt.Fprintln(os.Stderr, "bench:", err)
			return 1
		}
		fmt.Println(string(data))
	} else if *markdown {
		printMarkdown(value)
	} else {
		printHuman(value)
	}
	if gateFailed || hasFailure(value) {
		return 1
	}
	return 0
}

func loadCorpus(path string) (corpusContext, error) {
	info, err := os.Stat(path)
	if err != nil {
		return corpusContext{}, fmt.Errorf("corpus %s: %w", path, err)
	}
	if !info.IsDir() {
		return corpusContext{}, fmt.Errorf("corpus %s is not a directory", path)
	}
	name := filepath.Base(path)
	var summary benchutil.ProfileSummary
	if manifest, err := benchutil.LoadManifest(); err == nil {
		summary = manifest.Profiles[name]
	}
	bundle, err := okf.ParseBundle(context.Background(), path, okf.Options{Cache: false})
	if err != nil {
		return corpusContext{}, fmt.Errorf("parse corpus: %w", err)
	}
	return corpusContext{Path: path, Name: name, Summary: summary, Bundle: bundle}, nil
}

func newReport(corpus corpusContext) report {
	return report{
		Schema:     schemaVersion,
		Commit:     commitHash(),
		Go:         runtime.Version(),
		Runner:     runtime.GOOS + "/" + runtime.GOARCH,
		GOMAXPROCS: runtime.GOMAXPROCS(0),
		Corpus: corpusInfo{
			Name:      corpus.Name,
			SHA256:    corpus.Summary.SHA256,
			Docs:      corpus.Summary.Documents,
			BodyBytes: corpus.Summary.BodyBytes,
			Edges:     corpus.Summary.Edges,
		},
	}
}

func commitHash() string {
	command := exec.Command("git", "rev-parse", "HEAD")
	output, err := command.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

func scenarioNames(value string) []string {
	if value == "all" {
		return []string{"parse", "ingest", "sync", "search", "explain", "audit", "export", "space"}
	}
	names := strings.Split(value, ",")
	for index := range names {
		names[index] = strings.TrimSpace(names[index])
	}
	return names
}

func runScenarios(
	value *report,
	corpus corpusContext,
	registry map[string]benchutil.RegistryEntry,
	queries benchutil.QuerySet,
	names []string,
	samples int,
	weaveBinary string,
) error {
	ranWrites := false
	ranCLI := false
	for _, name := range names {
		switch name {
		case "parse":
			if err := runParse(value, corpus, registry, samples); err != nil {
				return err
			}
		case "ingest", "sync":
			if ranWrites {
				continue
			}
			if err := runIngestSync(value, corpus, registry, queries, samples); err != nil {
				return err
			}
			ranWrites = true
		case "search", "explain", "audit", "export":
			if ranCLI {
				continue
			}
			if err := runCLI(value, corpus, registry, queries, samples, weaveBinary); err != nil {
				return err
			}
			ranCLI = true
		case "space":
			if err := runSpace(value, corpus, registry, weaveBinary); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown scenario %q", name)
		}
	}
	return nil
}

func loadRegistry() (map[string]benchutil.RegistryEntry, error) {
	value, err := benchutil.LoadRegistry()
	if err != nil {
		return nil, err
	}
	entries := make(map[string]benchutil.RegistryEntry, len(value.Entries))
	for _, entry := range value.Entries {
		entries[entry.ID] = entry
	}
	return entries, nil
}

func (value *report) add(
	registry map[string]benchutil.RegistryEntry,
	id string,
	samples []float64,
	fallbackUnit string,
) {
	entry, ok := registry[id]
	if !ok {
		value.Results = append(value.Results, result{
			ID:      id,
			Unit:    fallbackUnit,
			Value:   0,
			Samples: samples,
			Status:  "skip",
		})
		return
	}
	aggregate := entry.Aggregate
	valueFloat := aggregateSamples(aggregate, samples)
	status := "baseline"
	if entry.Budget != nil {
		status = "pass"
		if valueFloat > *entry.Budget {
			status = "fail"
		}
	}
	value.Results = append(value.Results, result{
		ID:        id,
		Unit:      entry.Unit,
		Aggregate: aggregate,
		Value:     valueFloat,
		Samples:   samples,
		Budget:    entry.Budget,
		Status:    status,
	})
}

func (value *report) skip(registry map[string]benchutil.RegistryEntry, id string) {
	entry, ok := registry[id]
	if !ok {
		return
	}
	value.Results = append(value.Results, result{
		ID:        id,
		Unit:      entry.Unit,
		Aggregate: entry.Aggregate,
		Status:    "skip",
		Budget:    entry.Budget,
	})
}

func aggregateSamples(aggregate string, samples []float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	sorted := append([]float64(nil), samples...)
	sort.Float64s(sorted)
	switch aggregate {
	case "p50":
		return percentile(sorted, 0.50)
	case "p95":
		return percentile(sorted, 0.95)
	case "ratio", "value":
		return sorted[len(sorted)/2]
	default:
		return sorted[len(sorted)/2]
	}
}

func percentile(sorted []float64, fraction float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	index := int(fraction*float64(len(sorted))+0.999999) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

func durationMS(start time.Time) float64 {
	return float64(time.Since(start).Microseconds()) / 1000
}

func durationSeconds(start time.Time) float64 {
	return time.Since(start).Seconds()
}

func hasFailure(value report) bool {
	for _, item := range value.Results {
		if item.Status == "fail" {
			return true
		}
	}
	return false
}

func printHuman(value report) {
	fmt.Printf("bench %s corpus=%s docs=%d body=%d\n", value.Commit, value.Corpus.Name, value.Corpus.Docs, value.Corpus.BodyBytes)
	for _, item := range value.Results {
		budget := "-"
		if item.Budget != nil {
			budget = fmt.Sprintf("%.3f", *item.Budget)
		}
		fmt.Printf("%-34s %10.3f %-5s budget=%-8s %s\n", item.ID, item.Value, item.Unit, budget, item.Status)
	}
}

func printMarkdown(value report) {
	fmt.Printf("| ID | Value | Unit | Budget | Status |\n| --- | ---: | --- | ---: | --- |\n")
	for _, item := range value.Results {
		budget := "-"
		if item.Budget != nil {
			budget = fmt.Sprintf("%.3f", *item.Budget)
		}
		fmt.Printf("| `%s` | %.3f | %s | %s | %s |\n", item.ID, item.Value, item.Unit, budget, item.Status)
	}
}

func compareBaseline(value *report, path string, registry map[string]benchutil.RegistryEntry) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var old report
	if err := json.Unmarshal(data, &old); err != nil {
		return err
	}
	previous := make(map[string]float64, len(old.Results))
	for _, item := range old.Results {
		previous[item.ID] = item.Value
	}
	current := make(map[string]bool, len(value.Results))
	for _, item := range value.Results {
		current[item.ID] = true
	}
	failures := make([]string, 0)
	for id := range previous {
		if entry, ok := registry[id]; ok && entry.Blocking && !current[id] {
			failures = append(failures, fmt.Sprintf("%s: missing result", id))
		}
	}
	for index := range value.Results {
		item := &value.Results[index]
		entry, ok := registry[item.ID]
		if !ok || !entry.Blocking || item.Status == "skip" {
			continue
		}
		before, ok := previous[item.ID]
		if !ok || before <= 0 {
			continue
		}
		if item.Value > before*1.10 {
			item.Status = "fail"
			failures = append(failures, fmt.Sprintf("%s: %.3f > %.3f (+%.1f%%)",
				item.ID, item.Value, before, (item.Value/before-1)*100))
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("blocking regression: %s", strings.Join(failures, "; "))
	}
	return nil
}
