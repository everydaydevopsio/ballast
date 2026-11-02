package main

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// releaseDownloadBaseURL is the host serving published release assets. It is a
// variable so tests can point the installer at a local HTTP server.
var releaseDownloadBaseURL = "https://github.com/everydaydevopsio/ballast/releases/download"

// releaseDownloadTimeout bounds a single asset download so an unreachable or
// hung mirror fails with a clear error instead of blocking the install.
const releaseDownloadTimeout = 2 * time.Minute

// goBackendBinaryName is the file name inside the published archive and the
// name the wrapper installs into .ballast/bin.
const goBackendBinaryName = "ballast-go"

// installReleasedGoBinary downloads the published ballast-go archive for the
// running platform, verifies its SHA-256 against the release checksum file,
// extracts the binary, and installs it into .ballast/bin.
//
// This is implemented with the Go standard library rather than a shell or
// PowerShell pipeline so it works in minimal containers and locked-down CI
// images that have no curl, tar, openssl, install, or PowerShell available.
func installReleasedGoBinary(version string, projectRoot string) error {
	release := releaseVersion(version)
	if release == "" {
		release = releaseVersion(resolveVersion())
	}
	if release == "" {
		return errors.New("ballast-go install requires a release version or a ballast source checkout")
	}

	goos, goarch, archiveExt, err := releasedGoArchiveParts(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}

	archiveName := releasedGoArchiveName(release, goos, goarch, archiveExt)
	archive, err := downloadReleaseAsset(releasedGoArchiveURL(release, archiveName))
	if err != nil {
		return err
	}
	checksums, err := downloadReleaseAsset(releasedGoChecksumURL(release))
	if err != nil {
		return err
	}
	if err := verifyReleaseChecksum(archive, checksums, archiveName); err != nil {
		return err
	}

	binaryName := goBackendBinaryName
	if goos == "windows" {
		binaryName += ".exe"
	}
	payload, err := extractReleaseBinary(archive, archiveExt, binaryName)
	if err != nil {
		return err
	}

	destination := filepath.Join(projectRoot, ".ballast", "bin", binaryName)
	return writeExecutable(destination, payload)
}

// downloadReleaseAsset fetches a release asset into memory. Release archives
// are a few tens of megabytes, so buffering avoids a temporary file and the
// cleanup it would need.
func downloadReleaseAsset(url string) ([]byte, error) {
	client := &http.Client{Timeout: releaseDownloadTimeout}
	response, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: unexpected response status %s", url, response.Status)
	}

	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	return payload, nil
}

// verifyReleaseChecksum confirms the archive matches the digest published for
// it. A missing entry is a failure rather than a skip: an archive the release
// does not vouch for must not be installed.
func verifyReleaseChecksum(archive []byte, checksums []byte, archiveName string) error {
	expected, err := releaseChecksumFor(checksums, archiveName)
	if err != nil {
		return err
	}

	actual := fmt.Sprintf("%x", sha256.Sum256(archive))
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf(
			"checksum mismatch for %s: release publishes %s but the download hashed to %s",
			archiveName,
			strings.ToLower(expected),
			actual,
		)
	}
	return nil
}

// releaseChecksumFor reads the digest recorded for archiveName from a
// GoReleaser checksum file, whose lines are "<sha256>  <filename>".
func releaseChecksumFor(checksums []byte, archiveName string) (string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(checksums))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			continue
		}
		// Some checksum writers mark binary-mode entries with a "*" prefix.
		if strings.TrimPrefix(fields[1], "*") == archiveName {
			return fields[0], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read release checksum file: %w", err)
	}
	return "", fmt.Errorf("release checksum file has no entry for %s", archiveName)
}

func extractReleaseBinary(archive []byte, archiveExt string, binaryName string) ([]byte, error) {
	switch archiveExt {
	case "tar.gz":
		return extractTarGzEntry(archive, binaryName)
	case "zip":
		return extractZipEntry(archive, binaryName)
	default:
		return nil, fmt.Errorf("unsupported ballast-go archive format: %s", archiveExt)
	}
}

func extractTarGzEntry(archive []byte, name string) ([]byte, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("open release archive: %w", err)
	}
	defer func() { _ = gzipReader.Close() }()

	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read release archive: %w", err)
		}
		// Match on the base name only. The archive path is never used as a
		// write destination, so a crafted entry cannot escape .ballast/bin.
		if header.Typeflag != tar.TypeReg || path.Base(header.Name) != name {
			continue
		}
		payload, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("read %s from release archive: %w", name, err)
		}
		return payload, nil
	}
	return nil, fmt.Errorf("release archive does not contain %s", name)
}

func extractZipEntry(archive []byte, name string) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("open release archive: %w", err)
	}

	for _, file := range reader.File {
		if file.FileInfo().IsDir() || path.Base(file.Name) != name {
			continue
		}
		entry, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open %s in release archive: %w", name, err)
		}
		payload, readErr := io.ReadAll(entry)
		closeErr := entry.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read %s from release archive: %w", name, readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("read %s from release archive: %w", name, closeErr)
		}
		return payload, nil
	}
	return nil, fmt.Errorf("release archive does not contain %s", name)
}

// writeExecutable installs payload at destination with the executable bit set.
// It writes a sibling temporary file and renames it so a failed write never
// leaves a truncated binary in place, and so replacing a backend that is
// currently running succeeds.
func writeExecutable(destination string, payload []byte) error {
	directory := filepath.Dir(destination)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", directory, err)
	}

	temporary, err := os.CreateTemp(directory, ".ballast-go-*")
	if err != nil {
		return fmt.Errorf("create a temporary file in %s: %w", directory, err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()

	if _, err := temporary.Write(payload); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write %s: %w", destination, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("write %s: %w", destination, err)
	}
	if err := os.Chmod(temporaryPath, 0o755); err != nil {
		return fmt.Errorf("set permissions on %s: %w", destination, err)
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return fmt.Errorf("install %s: %w", destination, err)
	}
	return nil
}

func releasedGoArchiveName(release string, goos string, goarch string, archiveExt string) string {
	return fmt.Sprintf("ballast-go_%s_%s_%s.%s", release, goos, goarch, archiveExt)
}

func releasedGoArchiveURL(release string, archiveName string) string {
	return fmt.Sprintf("%s/v%s/%s", releaseDownloadBaseURL, release, archiveName)
}

func releasedGoChecksumURL(release string) string {
	return fmt.Sprintf("%s/v%s/ballast-go_checksums.txt", releaseDownloadBaseURL, release)
}

// releasedGoArchiveParts maps the running platform onto the published asset
// naming scheme. It must stay aligned with the goos/goarch matrix in
// .goreleaser.yaml for the ballast-go build.
func releasedGoArchiveParts(goos string, goarch string) (string, string, string, error) {
	switch goos {
	case "linux", "darwin":
		switch goarch {
		case "amd64", "arm64":
			return goos, goarch, "tar.gz", nil
		}
	case "windows":
		switch goarch {
		case "amd64":
			return goos, goarch, "zip", nil
		}
	}
	return "", "", "", fmt.Errorf(
		"no ballast-go release is published for %s/%s (released platforms: %s); "+
			"build the backend from a ballast source checkout instead",
		goos,
		goarch,
		strings.Join(releasedGoPlatforms(), ", "),
	)
}

func releasedGoPlatforms() []string {
	return []string{"linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64", "windows/amd64"}
}
