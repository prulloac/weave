// Package release builds release archives, checksums and provenance records.
package release

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var versionPattern = regexp.MustCompile(`^(?:v)?(\d+\.\d+\.\d+)(?:[-+][0-9A-Za-z][0-9A-Za-z.-]*)?$`)

// NormalizeVersion validates a tag and strips its leading v.
func NormalizeVersion(tag string) (string, error) {
	trimmed := strings.TrimSpace(tag)
	match := versionPattern.FindStringSubmatch(trimmed)
	if match == nil {
		return "", fmt.Errorf("invalid release version %q", tag)
	}
	return strings.TrimPrefix(trimmed, "v"), nil
}

// ArtifactName returns the archive name for one target.
func ArtifactName(version, goos, goarch string) string {
	extension := ".tar.gz"
	if goos == "windows" {
		extension = ".zip"
	}
	return fmt.Sprintf("weave_%s_%s_%s%s", version, goos, goarch, extension)
}

// PackageOptions describes one archive build.
type PackageOptions struct {
	Version string
	GOOS    string
	GOARCH  string
	Binary  string
	Root    string
	Out     string
}

// Package archives one binary with README and LICENSE.
func Package(options PackageOptions) (string, error) {
	version, err := NormalizeVersion(options.Version)
	if err != nil {
		return "", err
	}
	if options.GOOS == "" || options.GOARCH == "" {
		return "", fmt.Errorf("goos and goarch are required")
	}
	if options.Binary == "" || options.Root == "" || options.Out == "" {
		return "", fmt.Errorf("binary, root and out are required")
	}
	if err := os.MkdirAll(options.Out, 0o755); err != nil {
		return "", err
	}
	archive := filepath.Join(options.Out, ArtifactName(version, options.GOOS, options.GOARCH))

	binary, err := os.ReadFile(options.Binary)
	if err != nil {
		return "", err
	}
	readme, err := os.ReadFile(filepath.Join(options.Root, "README.md"))
	if err != nil {
		return "", err
	}
	binaryName := "weave"
	if options.GOOS == "windows" {
		binaryName = "weave.exe"
	}
	files := []archiveEntry{
		{Name: binaryName, Body: binary, Mode: 0o755},
		{Name: "README.md", Body: readme, Mode: 0o644},
	}
	if license, err := os.ReadFile(filepath.Join(options.Root, "LICENSE")); err == nil {
		files = append(files, archiveEntry{Name: "LICENSE", Body: license, Mode: 0o644})
	} else if !os.IsNotExist(err) {
		return "", err
	}

	if options.GOOS == "windows" {
		err = writeZip(archive, files)
	} else {
		err = writeTarGz(archive, files)
	}
	if err != nil {
		return "", err
	}
	return archive, nil
}

type archiveEntry struct {
	Name string
	Body []byte
	Mode os.FileMode
}

func writeTarGz(path string, files []archiveEntry) error {
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	for _, file := range files {
		header := &tar.Header{
			Name:     file.Name,
			Mode:     int64(file.Mode.Perm()),
			Size:     int64(len(file.Body)),
			ModTime:  time.Unix(0, 0).UTC(),
			Typeflag: tar.TypeReg,
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if _, err := tarWriter.Write(file.Body); err != nil {
			return err
		}
	}
	if err := tarWriter.Close(); err != nil {
		return err
	}
	if err := gzipWriter.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buffer.Bytes(), 0o644)
}

func writeZip(path string, files []archiveEntry) error {
	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	for _, file := range files {
		header := &zip.FileHeader{Name: file.Name, Method: zip.Deflate}
		header.SetMode(file.Mode)
		header.Modified = time.Unix(0, 0).UTC()
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}
		if _, err := writer.Write(file.Body); err != nil {
			return err
		}
	}
	if err := zipWriter.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buffer.Bytes(), 0o644)
}

// WriteChecksums writes a sorted sha256 manifest for archives in outDir.
func WriteChecksums(outDir, version string) (string, error) {
	normalized, err := NormalizeVersion(version)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return "", err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, "_checksums.txt") || strings.HasSuffix(name, "_buildinfo.txt") {
			continue
		}
		if !strings.HasSuffix(name, ".tar.gz") && !strings.HasSuffix(name, ".zip") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	var builder strings.Builder
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(outDir, name))
		if err != nil {
			return "", err
		}
		sum := sha256.Sum256(data)
		fmt.Fprintf(&builder, "%s  %s\n", hex.EncodeToString(sum[:]), name)
	}
	path := filepath.Join(outDir, fmt.Sprintf("weave_%s_checksums.txt", normalized))
	if err := os.WriteFile(path, []byte(builder.String()), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// BuildInfo renders deterministic provenance fields.
func BuildInfo(version, commit, goVersion string) string {
	builtAt := time.Now().UTC().Format(time.RFC3339)
	return fmt.Sprintf("version=%s\ncommit=%s\ngo=%s\nbuilt_at=%s\n", version, commit, goVersion, builtAt)
}

// WriteBuildInfo writes the provenance file into outDir.
func WriteBuildInfo(outDir, version, commit, goVersion string) (string, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(outDir, "weave_"+strings.TrimPrefix(version, "v")+"_buildinfo.txt")
	if err := os.WriteFile(path, []byte(BuildInfo(version, commit, goVersion)), 0o644); err != nil {
		return "", err
	}
	return path, nil
}
