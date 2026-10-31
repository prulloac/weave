package main

import (
	"fmt"
	"path/filepath"
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

func buildSourceSpecs(primary string, extras []string) ([]contractv1.SourceSpec, error) {
	alias, err := deriveSourceAlias(primary)
	if err != nil {
		return nil, err
	}
	specs := []contractv1.SourceSpec{{Alias: alias, Path: primary}}
	aliases := map[string]bool{alias: true}
	paths := map[string]bool{canonicalSourcePath(primary): true}

	for _, extra := range extras {
		alias := ""
		path := extra
		if index := strings.Index(extra, "="); index > 0 {
			alias = extra[:index]
			path = extra[index+1:]
		}
		if strings.TrimSpace(path) == "" {
			return nil, fmt.Errorf("--source requires a path")
		}
		if alias == "" {
			alias, err = deriveSourceAlias(path)
			if err != nil {
				return nil, err
			}
		} else if !validSourceAlias(alias) {
			return nil, fmt.Errorf("invalid source alias %q", alias)
		}
		if aliases[alias] {
			return nil, fmt.Errorf("duplicate source alias %q", alias)
		}
		canonical := canonicalSourcePath(path)
		if paths[canonical] {
			return nil, fmt.Errorf("duplicate source path %q", path)
		}
		aliases[alias] = true
		paths[canonical] = true
		specs = append(specs, contractv1.SourceSpec{Alias: alias, Path: path})
	}
	return specs, nil
}

func deriveSourceAlias(path string) (string, error) {
	base := strings.ToLower(filepath.Base(filepath.Clean(path)))
	var builder strings.Builder
	lastDash := false
	for _, char := range base {
		switch {
		case char >= 'a' && char <= 'z', char >= '0' && char <= '9':
			builder.WriteRune(char)
			lastDash = false
		default:
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	alias := strings.Trim(builder.String(), "-")
	if alias == "" {
		alias = "bundle"
	}
	if alias == "contract" || alias == "weave" {
		alias = "bundle"
	}
	return alias, nil
}

func validSourceAlias(alias string) bool {
	for index, char := range alias {
		switch {
		case char >= 'a' && char <= 'z', char >= '0' && char <= '9':
		case char == '-' && index > 0:
		default:
			return false
		}
	}
	return alias != ""
}

func canonicalSourcePath(path string) string {
	if absolute, err := filepath.Abs(path); err == nil {
		return filepath.Clean(absolute)
	}
	return filepath.Clean(path)
}
