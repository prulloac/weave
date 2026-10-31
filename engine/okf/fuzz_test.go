package okf

import "testing"

func FuzzExtractFrontmatter(f *testing.F) {
	f.Add("---\ntype: Concept\nkeywords:\n  - one\n---\nbody")
	f.Add("")
	f.Add("---\ntype: [\n---\n")
	f.Fuzz(func(t *testing.T, input string) {
		_, _, _, _ = ExtractFrontmatter(input)
	})
}

func FuzzExtractLinks(f *testing.F) {
	f.Add("[one](one.md)")
	f.Add("`[one](one.md)`")
	f.Add("[[nested]](target.md)")
	f.Fuzz(func(t *testing.T, input string) {
		_ = ExtractLinks(input)
	})
}

func FuzzConceptID(f *testing.F) {
	f.Add("concepts/one.md")
	f.Add("../escape.md")
	f.Add("")
	f.Fuzz(func(t *testing.T, input string) {
		_ = conceptID(input)
	})
}
