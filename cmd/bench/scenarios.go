package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/prulloac/weave/benchmarks"
	"github.com/prulloac/weave/benchmarks/benchutil"
	"github.com/prulloac/weave/engine"
	"github.com/prulloac/weave/engine/okf"
	sqlitestore "github.com/prulloac/weave/storage/sqlite"
	_ "modernc.org/sqlite"
)

func runParse(value *report, corpus corpusContext, registry map[string]benchutil.RegistryEntry, samples int) error {
	measure := func(cache bool, cacheDir string) ([]float64, error) {
		times := make([]float64, 0, samples)
		for index := 0; index < samples; index++ {
			start := time.Now()
			bundle, err := okf.ParseBundle(context.Background(), corpus.Path, okf.Options{
				Cache: cache, CacheDir: cacheDir,
			})
			if err != nil {
				return nil, err
			}
			times = append(times, durationSeconds(start))
			if len(bundle.Concepts) == 0 {
				return nil, fmt.Errorf("parse: empty bundle")
			}
		}
		return times, nil
	}
	cold, err := measure(false, "")
	if err != nil {
		return err
	}
	id := "WEAVE-B-OKF-PARSE-" + strings.ToUpper(corpus.Name)
	switch corpus.Name {
	case "1k":
		id = benchmarks.OKFParse1K
	case "10k":
		id = benchmarks.OKFParse10K
	}
	value.add(registry, id, cold, "s")
	if corpus.Name != "10k" {
		return nil
	}
	value.add(registry, benchmarks.OKFParse10KCold, cold, "s")
	cacheDir, err := os.MkdirTemp("", "weave-bench-cache-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(cacheDir)
	if _, err := okf.ParseBundle(context.Background(), corpus.Path, okf.Options{
		Cache: true, CacheDir: cacheDir,
	}); err != nil {
		return err
	}
	warm, err := measure(true, cacheDir)
	if err != nil {
		return err
	}
	value.add(registry, benchmarks.OKFParse10KWarm, warm, "s")
	return nil
}

func runIngestSync(
	value *report,
	corpus corpusContext,
	registry map[string]benchutil.RegistryEntry,
	queries benchutil.QuerySet,
	samples int,
) error {
	if corpus.Name != "10k" {
		return nil
	}
	ctx := context.Background()
	root, err := os.MkdirTemp("", "weave-bench-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	storageSamples := samples
	if storageSamples > 2 {
		storageSamples = 2
	}

	ingestTimes := make([]float64, 0, storageSamples)
	for index := 0; index < storageSamples; index++ {
		dbPath := filepath.Join(root, fmt.Sprintf("ingest-%d.db", index))
		store, err := sqlitestore.Open(dbPath)
		if err != nil {
			return err
		}
		start := time.Now()
		if _, err := store.Ingest(ctx, corpus.Bundle, sqlitestore.IngestOptions{
			Mode: "strict", Codec: "zstd",
		}); err != nil {
			_ = store.Close()
			return err
		}
		ingestTimes = append(ingestTimes, durationSeconds(start))
		if err := store.Close(); err != nil {
			return err
		}
	}
	value.add(registry, benchmarks.StoreIngest10K, ingestTimes, "s")

	dbPath := filepath.Join(root, "index.db")
	store, err := sqlitestore.Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	if _, err := store.Ingest(ctx, corpus.Bundle, sqlitestore.IngestOptions{
		Mode: "strict", Codec: "zstd",
	}); err != nil {
		return err
	}

	loadTimes := make([]float64, 0, samples)
	for index := 0; index < samples; index++ {
		start := time.Now()
		if _, err := store.LoadBundle(ctx, corpus.Bundle.Root); err != nil {
			return err
		}
		loadTimes = append(loadTimes, durationMS(start))
	}
	value.add(registry, benchmarks.StoreLoad10K, loadTimes, "ms")

	selected := selectQueries(queries, "bm25", 10)
	bm25Times := make([]float64, 0, len(selected)*samples)
	explainTimes := make([]float64, 0, len(selected)*samples)
	for _, item := range selected {
		for sample := 0; sample < samples; sample++ {
			start := time.Now()
			if _, err := store.SearchBM25(ctx, corpus.Bundle.Root, item.Query, 10); err != nil {
				return err
			}
			bm25Times = append(bm25Times, durationMS(start))
			start = time.Now()
			if _, err := store.ExplainBM25(ctx, corpus.Bundle.Root, item.Query, 10, "doc-00001"); err != nil {
				return err
			}
			explainTimes = append(explainTimes, durationMS(start))
		}
	}
	value.add(registry, benchmarks.StoreBM2510K, bm25Times, "ms")
	value.add(registry, benchmarks.StoreExplain10K, explainTimes, "ms")

	syncTimes := make([]float64, 0, storageSamples)
	for index := 0; index < storageSamples; index++ {
		start := time.Now()
		if _, err := store.Sync(ctx, corpus.Bundle.Root, sqlitestore.SyncOptions{}); err != nil {
			return err
		}
		syncTimes = append(syncTimes, durationSeconds(start))
	}
	value.add(registry, benchmarks.StoreSync10KNone, syncTimes, "s")

	copyRoot := filepath.Join(root, "corpus-1pct")
	if err := copyDir(corpus.Path, copyRoot); err != nil {
		return err
	}
	copied, err := okf.ParseBundle(ctx, copyRoot, okf.Options{Cache: false})
	if err != nil {
		return err
	}
	copyDB := filepath.Join(root, "sync-1pct.db")
	copyStore, err := sqlitestore.Open(copyDB)
	if err != nil {
		return err
	}
	defer copyStore.Close()
	if _, err := copyStore.Ingest(ctx, copied, sqlitestore.IngestOptions{
		Mode: "strict", Codec: "zstd",
	}); err != nil {
		return err
	}
	if err := modifyPercent(copyRoot, 1); err != nil {
		return err
	}
	start := time.Now()
	if _, err := copyStore.Sync(ctx, copied.Root, sqlitestore.SyncOptions{}); err != nil {
		return err
	}
	value.add(registry, benchmarks.StoreSync10K1Pct, []float64{durationSeconds(start)}, "s")
	return nil
}

func runCLI(
	value *report,
	corpus corpusContext,
	registry map[string]benchutil.RegistryEntry,
	queries benchutil.QuerySet,
	samples int,
	weaveBinary string,
) error {
	if corpus.Name != "10k" {
		skipIDs(value, registry,
			benchmarks.CLIVersionCold, benchmarks.CLISearchKeyword, benchmarks.CLISearchFuzzy,
			benchmarks.CLISearchBM25, benchmarks.CLISearchHybrid, benchmarks.CLIExplain,
			benchmarks.CLIAudit, benchmarks.CLIExportRaw,
		)
		return nil
	}
	binary := weaveBinary
	if binary == "" {
		found, err := exec.LookPath("weave")
		if err != nil {
			for _, id := range []string{
				benchmarks.CLIVersionCold, benchmarks.CLISearchKeyword, benchmarks.CLISearchFuzzy,
				benchmarks.CLISearchBM25, benchmarks.CLISearchHybrid, benchmarks.CLIExplain,
				benchmarks.CLIAudit, benchmarks.CLIExportRaw,
			} {
				value.skip(registry, id)
			}
			return nil
		}
		binary = found
	}
	ctx := context.Background()
	root, err := os.MkdirTemp("", "weave-bench-cli-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	dbPath := filepath.Join(root, "index.db")
	store, err := sqlitestore.Open(dbPath)
	if err != nil {
		return err
	}
	if _, err := store.Ingest(ctx, corpus.Bundle, sqlitestore.IngestOptions{
		Mode: "strict", Codec: "zstd",
	}); err != nil {
		_ = store.Close()
		return err
	}
	if err := store.Close(); err != nil {
		return err
	}

	versionTimes := make([]float64, 0, samples)
	for index := 0; index < samples; index++ {
		elapsed, err := measureCLI(binary, "version", "--json")
		if err != nil {
			return err
		}
		versionTimes = append(versionTimes, elapsed)
	}
	value.add(registry, benchmarks.CLIVersionCold, versionTimes, "ms")

	modes := []struct {
		id   string
		mode string
	}{
		{benchmarks.CLISearchKeyword, "keyword"},
		{benchmarks.CLISearchFuzzy, "fuzzy"},
		{benchmarks.CLISearchBM25, "bm25"},
		{benchmarks.CLISearchHybrid, "hybrid"},
	}
	for _, item := range modes {
		queryLimit := 3
		modeSamples := samples
		if item.mode == "bm25" {
			queryLimit = 10
		} else {
			modeSamples = 1
		}
		selected := selectQueries(queries, item.mode, queryLimit)
		times := make([]float64, 0, len(selected)*modeSamples)
		for _, query := range selected {
			for sample := 0; sample < modeSamples; sample++ {
				elapsed, err := measureCLI(binary, "search", corpus.Path, query.Query,
					"--db", dbPath, "--mode", item.mode, "--json")
				if err != nil {
					return err
				}
				times = append(times, elapsed)
			}
		}
		value.add(registry, item.id, times, "ms")
	}

	explainTimes := make([]float64, 0)
	for _, query := range selectQueries(queries, "bm25", 3) {
		elapsed, err := measureCLI(binary, "explain", corpus.Path, query.Query,
			"--db", dbPath, "--json")
		if err != nil {
			return err
		}
		explainTimes = append(explainTimes, elapsed)
	}
	value.add(registry, benchmarks.CLIExplain, explainTimes, "ms")

	auditTimes := make([]float64, 0, 2)
	for index := 0; index < 2; index++ {
		elapsed, err := measureCLI(binary, "audit", corpus.Path,
			"--db", dbPath, "--fail-on", "none", "--json")
		if err != nil {
			return err
		}
		auditTimes = append(auditTimes, elapsed/1000)
	}
	value.add(registry, benchmarks.CLIAudit, auditTimes, "s")

	exportTimes := make([]float64, 0, 2)
	for index := 0; index < 2; index++ {
		out := filepath.Join(root, fmt.Sprintf("export-%d", index))
		elapsed, err := measureCLI(binary, "export", corpus.Path, "--out", out,
			"--format", "raw", "--yes", "--db", dbPath, "--json")
		if err != nil {
			return err
		}
		exportTimes = append(exportTimes, elapsed/1000)
	}
	value.add(registry, benchmarks.CLIExportRaw, exportTimes, "s")
	return nil
}

func runSpace(
	value *report,
	corpus corpusContext,
	registry map[string]benchutil.RegistryEntry,
	weaveBinary string,
) error {
	if corpus.Name != "10k" {
		skipIDs(value, registry,
			benchmarks.SpaceDB10K, benchmarks.SpaceDBWAL10K, benchmarks.SpaceDBFree10K,
			benchmarks.SpaceDBDocs10K, benchmarks.SpaceDBEdges10K, benchmarks.SpaceDBFTS10K,
			benchmarks.SpaceZstdRatio, benchmarks.SpaceBinary, benchmarks.MemIndex10K,
			benchmarks.MemPeakIngest10K,
		)
		return nil
	}
	ctx := context.Background()
	root, err := os.MkdirTemp("", "weave-bench-space-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	dbPath := filepath.Join(root, "index.db")
	store, err := sqlitestore.Open(dbPath)
	if err != nil {
		return err
	}
	if _, err := store.Ingest(ctx, corpus.Bundle, sqlitestore.IngestOptions{
		Mode: "strict", Codec: "zstd",
	}); err != nil {
		_ = store.Close()
		return err
	}
	if err := store.Close(); err != nil {
		return err
	}

	addValue := func(id string, valueFloat float64) {
		value.add(registry, id, []float64{valueFloat}, "MB")
	}
	if info, err := os.Stat(dbPath); err == nil {
		addValue(benchmarks.SpaceDB10K, float64(info.Size())/1024/1024)
	}
	if info, err := os.Stat(dbPath + "-wal"); err == nil {
		addValue(benchmarks.SpaceDBWAL10K, float64(info.Size())/1024/1024)
	} else {
		addValue(benchmarks.SpaceDBWAL10K, 0)
	}

	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	defer raw.Close()
	var pageSize, freePages int64
	if err := raw.QueryRow("PRAGMA page_size").Scan(&pageSize); err != nil {
		return err
	}
	if err := raw.QueryRow("PRAGMA freelist_count").Scan(&freePages); err != nil {
		return err
	}
	addValue(benchmarks.SpaceDBFree10K, float64(pageSize*freePages)/1024/1024)

	tables := []struct {
		id    string
		table string
	}{
		{benchmarks.SpaceDBDocs10K, "documents"},
		{benchmarks.SpaceDBEdges10K, "edges"},
		{benchmarks.SpaceDBFTS10K, "fts_documents"},
	}
	for _, item := range tables {
		var size int64
		var err error
		if item.id == benchmarks.SpaceDBFTS10K {
			err = raw.QueryRow("SELECT COALESCE(SUM(pgsize),0) FROM dbstat WHERE name LIKE 'fts_%'").Scan(&size)
		} else {
			err = raw.QueryRow("SELECT COALESCE(SUM(pgsize),0) FROM dbstat WHERE name = ?", item.table).Scan(&size)
		}
		if err != nil {
			value.skip(registry, item.id)
			continue
		}
		addValue(item.id, float64(size)/1024/1024)
	}

	var stored int64
	if err := raw.QueryRow("SELECT COALESCE(SUM(LENGTH(body_compressed)),0) FROM documents").Scan(&stored); err == nil && corpus.Summary.BodyBytes > 0 {
		value.add(registry, benchmarks.SpaceZstdRatio, []float64{
			float64(stored) / float64(corpus.Summary.BodyBytes),
		}, "ratio")
	}

	if weaveBinary != "" {
		if info, err := os.Stat(weaveBinary); err == nil {
			value.add(registry, benchmarks.SpaceBinary, []float64{
				float64(info.Size()) / 1024 / 1024,
			}, "MB")
		} else {
			value.skip(registry, benchmarks.SpaceBinary)
		}
	} else {
		value.skip(registry, benchmarks.SpaceBinary)
	}

	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	eng := engine.FromBundle(corpus.Bundle)
	eng.SearchIndex()
	runtime.GC()
	runtime.ReadMemStats(&after)
	if corpus.Summary.BodyBytes > 0 {
		delta := int64(after.HeapAlloc) - int64(before.HeapAlloc)
		if delta < 0 {
			delta = 0
		}
		value.add(registry, benchmarks.MemIndex10K, []float64{
			float64(delta) / float64(corpus.Summary.BodyBytes),
		}, "ratio")
	}
	runtime.KeepAlive(eng)

	if peak, ok := peakRSS(); ok {
		value.add(registry, benchmarks.MemPeakIngest10K, []float64{peak}, "MB")
	} else {
		value.skip(registry, benchmarks.MemPeakIngest10K)
	}
	return nil
}

func skipIDs(value *report, registry map[string]benchutil.RegistryEntry, ids ...string) {
	for _, id := range ids {
		value.skip(registry, id)
	}
}

func measureCLI(binary string, args ...string) (float64, error) {
	command := exec.Command(binary, args...)
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	start := time.Now()
	err := command.Run()
	elapsed := durationMS(start)
	if err == nil {
		return elapsed, nil
	}
	if exit, ok := err.(*exec.ExitError); ok {
		if exit.ExitCode() <= 1 {
			return elapsed, nil
		}
		return 0, fmt.Errorf("%s %s: exit %d", binary, strings.Join(args, " "), exit.ExitCode())
	}
	return 0, err
}

func selectQueries(queries benchutil.QuerySet, mode string, limit int) []benchutil.Query {
	selected := make([]benchutil.Query, 0, limit)
	for _, item := range queries.Queries {
		if item.Mode != mode {
			continue
		}
		selected = append(selected, item)
		if len(selected) == limit {
			break
		}
	}
	if len(selected) == 0 {
		selected = append(selected, queries.Queries[:min(limit, len(queries.Queries))]...)
	}
	return selected
}

func copyDir(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, data, 0o644)
	})
}

func modifyPercent(root string, percent int) error {
	files := make([]string, 0)
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		return err
	}
	count := len(files) * percent / 100
	if count < 1 {
		count = 1
	}
	for index := 0; index < count && index < len(files); index++ {
		file, err := os.OpenFile(files[index], os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		if _, err := file.WriteString("\n<!-- benchmark change -->\n"); err != nil {
			_ = file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	return nil
}

func peakRSS() (float64, bool) {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0, false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "VmHWM:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0, false
		}
		kb, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			return 0, false
		}
		return kb / 1024, true
	}
	return 0, false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
