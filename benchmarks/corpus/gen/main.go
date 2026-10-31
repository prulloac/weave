// Command gen writes the deterministic Weave benchmark corpora.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

const defaultSeed = 20261031

type profile struct {
	Name  string
	Docs  int
	Dirty bool
}

var profiles = []profile{
	{Name: "100", Docs: 100},
	{Name: "1k", Docs: 1000},
	{Name: "10k", Docs: 10000},
	{Name: "dirty-10k", Docs: 10000, Dirty: true},
}

type manifest struct {
	Seed     int64                     `json:"seed"`
	Profiles map[string]profileSummary `json:"profiles"`
}

type profileSummary struct {
	Documents int    `json:"documents"`
	BodyBytes int64  `json:"body_bytes"`
	Edges     int    `json:"edges"`
	Keywords  int    `json:"keywords"`
	SHA256    string `json:"sha256"`
}

type generatedDoc struct {
	content       string
	body          []byte
	keywords      []string
	resolvedLinks int
}

func main() {
	out := flag.String("out", "benchmarks/corpus", "corpus output directory")
	only := flag.String("only", "", "comma-separated profiles (default all)")
	seed := flag.Int64("seed", defaultSeed, "deterministic seed")
	verify := flag.Bool("verify", false, "regenerate twice and compare the manifest")
	flag.Parse()

	names := profileNames(*only)
	if *verify {
		if err := verifyCorpora(*out, names, *seed); err != nil {
			fmt.Fprintln(os.Stderr, "corpus gen:", err)
			os.Exit(1)
		}
		fmt.Println("corpus gen: deterministic")
		return
	}
	value, err := generateAll(*out, names, *seed)
	if err != nil {
		fmt.Fprintln(os.Stderr, "corpus gen:", err)
		os.Exit(1)
	}
	fmt.Printf("corpus gen: wrote %d profiles to %s\n", len(value.Profiles), *out)
}

func profileNames(only string) []string {
	if strings.TrimSpace(only) == "" {
		names := make([]string, len(profiles))
		for index, item := range profiles {
			names[index] = item.Name
		}
		return names
	}
	names := strings.Split(only, ",")
	for index := range names {
		names[index] = strings.TrimSpace(names[index])
	}
	return names
}

func generateAll(out string, names []string, seed int64) (manifest, error) {
	value := manifest{Seed: seed, Profiles: map[string]profileSummary{}}
	known := make(map[string]bool, len(profiles))
	for _, item := range profiles {
		known[item.Name] = true
	}
	selected := make(map[string]bool, len(names))
	for _, name := range names {
		if !known[name] {
			return manifest{}, fmt.Errorf("unknown profile %q", name)
		}
		selected[name] = true
	}
	for _, item := range profiles {
		if !selected[item.Name] {
			continue
		}
		summary, err := generateProfile(filepath.Join(out, item.Name), item, seed)
		if err != nil {
			return manifest{}, err
		}
		value.Profiles[item.Name] = summary
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return manifest{}, err
	}
	data, err := json.MarshalIndent(value, "", "\t")
	if err != nil {
		return manifest{}, err
	}
	if err := os.WriteFile(filepath.Join(out, "manifest.json"), append(data, '\n'), 0o644); err != nil {
		return manifest{}, err
	}
	return value, nil
}

func generateProfile(dir string, item profile, seed int64) (profileSummary, error) {
	if err := os.RemoveAll(dir); err != nil {
		return profileSummary{}, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return profileSummary{}, err
	}
	random := rand.New(rand.NewPCG(uint64(seed), uint64(seed+int64(item.Docs))))
	orphans := make(map[int]bool, item.Docs/20+1)
	for index := 0; index < item.Docs; index += 20 {
		orphans[index] = true
	}
	keywords := make(map[string]bool)
	summary := profileSummary{Documents: item.Docs}
	for index := 0; index < item.Docs; index++ {
		doc := buildDocument(random, item, index, orphans)
		name := fmt.Sprintf("doc-%05d.md", index)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(doc.content), 0o644); err != nil {
			return profileSummary{}, err
		}
		summary.BodyBytes += int64(len(doc.body))
		summary.Edges += doc.resolvedLinks
		for _, keyword := range doc.keywords {
			keywords[keyword] = true
		}
	}
	summary.Keywords = len(keywords)
	hash, err := hashDir(dir)
	if err != nil {
		return profileSummary{}, err
	}
	summary.SHA256 = hash
	return summary, nil
}

func buildDocument(random *rand.Rand, item profile, index int, orphans map[int]bool) generatedDoc {
	keywordZipf := rand.NewZipf(random, 1.1, 1, 499)
	bodyZipf := rand.NewZipf(random, 1.1, 1, 1999)
	orphan := orphans[index]

	keywords := make([]string, 0, 6)
	seen := make(map[string]bool)
	targetKeywords := 3 + random.IntN(4)
	for len(keywords) < targetKeywords {
		keyword := fmt.Sprintf("kw%03d", keywordZipf.Uint64())
		if seen[keyword] {
			continue
		}
		seen[keyword] = true
		keywords = append(keywords, keyword)
	}

	description := makeDescription(random, bodyZipf, 60+random.IntN(101))
	body := buildBody(random, bodyZipf, index)

	related := make([]string, 0)
	inline := make([]string, 0)
	resolved := 0
	if !orphan {
		linkCount := random.IntN(6)
		for link := 0; link < linkCount; link++ {
			target := random.IntN(item.Docs)
			for orphans[target] && item.Docs > 1 {
				target = (target + 1) % item.Docs
			}
			dangling := random.IntN(100) < 10
			path := fmt.Sprintf("doc-%05d.md", target)
			if dangling {
				path = fmt.Sprintf("missing-%05d.md", random.IntN(item.Docs))
			} else {
				resolved++
			}
			if random.IntN(100) < 40 {
				related = append(related, path)
			} else {
				inline = append(inline, path)
			}
		}
	}
	var out strings.Builder
	out.WriteString("---\n")
	out.WriteString("type: Concept\n")
	fmt.Fprintf(&out, "title: \"Doc %05d\"\n", index)
	fmt.Fprintf(&out, "description: \"%s\"\n", description)
	if !(item.Dirty && index%50 == 0) {
		out.WriteString("keywords:\n")
		for _, keyword := range keywords {
			fmt.Fprintf(&out, "  - %s\n", keyword)
		}
	}
	if random.IntN(100) < 10 {
		out.WriteString("tags:\n  - bench\n")
	}
	if random.IntN(100) < 10 {
		status := "stable"
		if random.IntN(2) == 0 {
			status = "draft"
		}
		fmt.Fprintf(&out, "status: %s\n", status)
	}
	if len(related) > 0 {
		out.WriteString("related:\n")
		for _, path := range related {
			fmt.Fprintf(&out, "  - %s\n", path)
		}
	}
	out.WriteString("---\n\n")
	out.Write(body)
	if len(inline) > 0 {
		out.WriteString("\nSee ")
		for linkIndex, path := range inline {
			if linkIndex > 0 {
				out.WriteString(", ")
			}
			fmt.Fprintf(&out, "[%s](%s)", strings.TrimSuffix(path, ".md"), path)
		}
		out.WriteString(".\n")
	}
	if item.Dirty && index%100 == 1 {
		// Malformed YAML keeps the file in the parse-error path.
		content := strings.Replace(out.String(), "keywords:\n", "keywords: [unclosed\n", 1)
		return generatedDoc{content: content, body: body, keywords: keywords, resolvedLinks: resolved}
	}
	return generatedDoc{content: out.String(), body: body, keywords: keywords, resolvedLinks: resolved}
}

func buildBody(random *rand.Rand, zipf *rand.Zipf, index int) []byte {
	var body strings.Builder
	fmt.Fprintf(&body, "# Doc %05d\n\n", index)
	target := 1024 + random.IntN(3073)
	for body.Len() < target {
		sentences := 2 + random.IntN(3)
		for sentence := 0; sentence < sentences; sentence++ {
			words := 8 + random.IntN(9)
			for word := 0; word < words; word++ {
				if word > 0 {
					body.WriteByte(' ')
				}
				body.WriteString(fmt.Sprintf("term%04d", zipf.Uint64()))
			}
			body.WriteString(". ")
		}
		body.WriteString("\n\n")
	}
	return []byte(body.String())
}

func makeDescription(random *rand.Rand, zipf *rand.Zipf, target int) string {
	var description strings.Builder
	for description.Len() < target {
		if description.Len() > 0 {
			description.WriteByte(' ')
		}
		description.WriteString(fmt.Sprintf("term%04d", zipf.Uint64()))
	}
	return description.String()
}

func hashDir(dir string) (string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	hash := sha256.New()
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return "", err
		}
		hash.Write([]byte(filepath.ToSlash(relative)))
		hash.Write([]byte{0})
		hash.Write(data)
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func verifyCorpora(out string, names []string, seed int64) error {
	root, err := os.MkdirTemp("", "weave-corpus-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	first, err := generateAll(filepath.Join(root, "first"), names, seed)
	if err != nil {
		return err
	}
	second, err := generateAll(filepath.Join(root, "second"), names, seed)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(first, second) {
		return fmt.Errorf("generated manifests differ")
	}
	path := filepath.Join(out, "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read committed manifest: %w", err)
	}
	var recorded manifest
	if err := json.Unmarshal(data, &recorded); err != nil {
		return err
	}
	if !reflect.DeepEqual(first, recorded) {
		return fmt.Errorf("committed manifest is stale; run gen -out %s", out)
	}
	return nil
}
