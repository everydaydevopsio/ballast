package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
)

func tarGzArchive(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	for name, payload := range entries {
		header := &tar.Header{
			Name:     name,
			Mode:     0o755,
			Size:     int64(len(payload)),
			Typeflag: tar.TypeReg,
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := tarWriter.Write(payload); err != nil {
			t.Fatalf("write tar entry: %v", err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	return buffer.Bytes()
}

func zipArchive(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	for name, payload := range entries {
		writer, err := zipWriter.Create(name)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := writer.Write(payload); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	return buffer.Bytes()
}

func checksumLine(payload []byte, name string) string {
	return fmt.Sprintf("%x  %s\n", sha256.Sum256(payload), name)
}

// serveRelease stands up a local stand-in for the GitHub release host so the
// installer can be exercised end to end without network access.
func serveRelease(t *testing.T, assets map[string][]byte) {
	t.Helper()
	serveReleaseRecording(t, assets)
}

// serveReleaseRecording serves release assets and returns an accessor for the
// asset paths the installer requested, so tests can assert which release and
// platform the installer reached for.
func serveReleaseRecording(t *testing.T, assets map[string][]byte) func() []string {
	t.Helper()
	var mutex sync.Mutex
	var requested []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()
		requested = append(requested, r.URL.Path)
		mutex.Unlock()

		payload, ok := assets[path.Base(r.URL.Path)]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if _, err := w.Write(payload); err != nil {
			t.Errorf("serve release asset: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	originalBaseURL := releaseDownloadBaseURL
	t.Cleanup(func() { releaseDownloadBaseURL = originalBaseURL })
	releaseDownloadBaseURL = server.URL + "/releases/download"

	return func() []string {
		mutex.Lock()
		defer mutex.Unlock()
		return slices.Clone(requested)
	}
}

// serveReleaseForVersion publishes a synthetic ballast-go release for the
// running platform. It returns the installed binary name, the payload the
// release publishes, and an accessor for the requested asset paths.
func serveReleaseForVersion(t *testing.T, release string) (string, []byte, func() []string) {
	t.Helper()
	archiveName, binaryName := currentReleaseAssetNames(t, release)
	payload := []byte("ballast-go " + release)
	archive := currentReleaseArchive(t, binaryName, payload)

	requestedPaths := serveReleaseRecording(t, map[string][]byte{
		archiveName:                archive,
		"ballast-go_checksums.txt": []byte(checksumLine(archive, archiveName)),
	})
	return binaryName, payload, requestedPaths
}

// currentReleaseAssetNames returns the archive and binary names the installer
// looks for on the platform running the test.
func currentReleaseAssetNames(t *testing.T, release string) (string, string) {
	t.Helper()
	goos, goarch, archiveExt, err := releasedGoArchiveParts(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Skipf("no published ballast-go release for this platform: %v", err)
	}
	binaryName := goBackendBinaryName
	if goos == "windows" {
		binaryName += ".exe"
	}
	return releasedGoArchiveName(release, goos, goarch, archiveExt), binaryName
}

func currentReleaseArchive(t *testing.T, binaryName string, payload []byte) []byte {
	t.Helper()
	if runtime.GOOS == "windows" {
		return zipArchive(t, map[string][]byte{binaryName: payload})
	}
	return tarGzArchive(t, map[string][]byte{binaryName: payload})
}

// A verified release archive must land in .ballast/bin as an executable, with
// no external download or extraction tooling involved.
func TestInstallReleasedGoBinaryInstallsVerifiedArchive(t *testing.T) {
	release := "5.21.3"
	archiveName, binaryName := currentReleaseAssetNames(t, release)
	binaryPayload := []byte("#!/bin/sh\necho ballast-go\n")
	archive := currentReleaseArchive(t, binaryName, binaryPayload)

	serveRelease(t, map[string][]byte{
		archiveName:                archive,
		"ballast-go_checksums.txt": []byte(checksumLine(archive, archiveName)),
	})

	projectRoot := resolvedTempDir(t)
	if err := installReleasedGoBinary("v"+release, projectRoot); err != nil {
		t.Fatalf("unexpected install error: %v", err)
	}

	destination := filepath.Join(projectRoot, ".ballast", "bin", binaryName)
	installed, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("read installed binary: %v", err)
	}
	if !bytes.Equal(installed, binaryPayload) {
		t.Fatal("installed binary does not match the archive payload")
	}

	info, err := os.Stat(destination)
	if err != nil {
		t.Fatalf("stat installed binary: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("expected the installed binary to be executable, got mode %v", info.Mode().Perm())
	}
}

// A tampered or truncated download must not be installed.
func TestInstallReleasedGoBinaryRejectsChecksumMismatch(t *testing.T) {
	release := "5.21.3"
	archiveName, binaryName := currentReleaseAssetNames(t, release)
	archive := currentReleaseArchive(t, binaryName, []byte("real payload"))
	decoy := currentReleaseArchive(t, binaryName, []byte("tampered payload"))

	serveRelease(t, map[string][]byte{
		archiveName:                archive,
		"ballast-go_checksums.txt": []byte(checksumLine(decoy, archiveName)),
	})

	projectRoot := resolvedTempDir(t)
	err := installReleasedGoBinary("v"+release, projectRoot)
	if err == nil {
		t.Fatal("expected a checksum mismatch to fail the install")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected a checksum mismatch error, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(projectRoot, ".ballast", "bin", binaryName)); !os.IsNotExist(statErr) {
		t.Fatal("expected no binary to be installed after a checksum mismatch")
	}
}

// An archive the release publishes no checksum for must fail closed rather than
// install unverified.
func TestInstallReleasedGoBinaryRejectsMissingChecksumEntry(t *testing.T) {
	release := "5.21.3"
	archiveName, _ := currentReleaseAssetNames(t, release)
	archive := currentReleaseArchive(t, goBackendBinaryName, []byte("real payload"))

	serveRelease(t, map[string][]byte{
		archiveName:                archive,
		"ballast-go_checksums.txt": []byte("deadbeef  some-other-archive.tar.gz\n"),
	})

	err := installReleasedGoBinary("v"+release, resolvedTempDir(t))
	if err == nil {
		t.Fatal("expected a missing checksum entry to fail the install")
	}
	if !strings.Contains(err.Error(), "no entry for "+archiveName) {
		t.Fatalf("expected an error naming the missing archive, got %v", err)
	}
}

func TestInstallReleasedGoBinaryReportsMissingReleaseAsset(t *testing.T) {
	release := "5.21.3"
	currentReleaseAssetNames(t, release)
	serveRelease(t, map[string][]byte{})

	err := installReleasedGoBinary("v"+release, resolvedTempDir(t))
	if err == nil {
		t.Fatal("expected a missing release asset to fail the install")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Fatalf("expected the response status in the error, got %v", err)
	}
}

func TestInstallReleasedGoBinaryRequiresReleaseVersion(t *testing.T) {
	originalVersion := version
	t.Cleanup(func() { version = originalVersion })
	version = "dev"

	err := installReleasedGoBinary("dev", resolvedTempDir(t))
	if err == nil {
		t.Fatal("expected a dev version with no release to fail")
	}
	if !strings.Contains(err.Error(), "requires a release version") {
		t.Fatalf("expected a release-version error, got %v", err)
	}
}

func TestReleasedGoArchivePartsCoversPublishedPlatforms(t *testing.T) {
	cases := []struct {
		goos       string
		goarch     string
		archiveExt string
	}{
		{"linux", "amd64", "tar.gz"},
		{"linux", "arm64", "tar.gz"},
		{"darwin", "amd64", "tar.gz"},
		{"darwin", "arm64", "tar.gz"},
		{"windows", "amd64", "zip"},
	}

	for _, testCase := range cases {
		t.Run(testCase.goos+"/"+testCase.goarch, func(t *testing.T) {
			goos, goarch, archiveExt, err := releasedGoArchiveParts(testCase.goos, testCase.goarch)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if goos != testCase.goos || goarch != testCase.goarch {
				t.Fatalf("expected %s/%s, got %s/%s", testCase.goos, testCase.goarch, goos, goarch)
			}
			if archiveExt != testCase.archiveExt {
				t.Fatalf("expected %s archives, got %s", testCase.archiveExt, archiveExt)
			}
		})
	}
}

// Unsupported platforms must say which platforms are published and what to do
// instead, rather than failing with a bare download error later on.
func TestReleasedGoArchivePartsRejectsUnpublishedPlatforms(t *testing.T) {
	cases := [][2]string{
		{"windows", "arm64"},
		{"linux", "386"},
		{"freebsd", "amd64"},
		{"plan9", "arm64"},
	}

	for _, testCase := range cases {
		t.Run(testCase[0]+"/"+testCase[1], func(t *testing.T) {
			_, _, _, err := releasedGoArchiveParts(testCase[0], testCase[1])
			if err == nil {
				t.Fatalf("expected %s/%s to be unsupported", testCase[0], testCase[1])
			}
			message := err.Error()
			if !strings.Contains(message, testCase[0]+"/"+testCase[1]) {
				t.Fatalf("expected the error to name the platform, got %v", err)
			}
			if !strings.Contains(message, "linux/amd64") {
				t.Fatalf("expected the error to list published platforms, got %v", err)
			}
			if !strings.Contains(message, "source checkout") {
				t.Fatalf("expected the error to suggest a source build, got %v", err)
			}
		})
	}
}

func TestReleaseChecksumForParsesChecksumFile(t *testing.T) {
	checksums := []byte(strings.Join([]string{
		"1111111111111111111111111111111111111111111111111111111111111111  ballast-go_5.21.3_linux_amd64.tar.gz",
		"2222222222222222222222222222222222222222222222222222222222222222 *ballast-go_5.21.3_darwin_arm64.tar.gz",
		"",
		"not a checksum line at all",
	}, "\n"))

	for name, expected := range map[string]string{
		"ballast-go_5.21.3_linux_amd64.tar.gz":  "1111111111111111111111111111111111111111111111111111111111111111",
		"ballast-go_5.21.3_darwin_arm64.tar.gz": "2222222222222222222222222222222222222222222222222222222222222222",
	} {
		checksum, err := releaseChecksumFor(checksums, name)
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", name, err)
		}
		if checksum != expected {
			t.Fatalf("expected %s for %s, got %s", expected, name, checksum)
		}
	}
}

func TestVerifyReleaseChecksumAcceptsUppercaseDigest(t *testing.T) {
	archive := []byte("payload")
	digest := strings.ToUpper(fmt.Sprintf("%x", sha256.Sum256(archive)))
	checksums := []byte(digest + "  archive.tar.gz\n")

	if err := verifyReleaseChecksum(archive, checksums, "archive.tar.gz"); err != nil {
		t.Fatalf("expected an uppercase digest to verify, got %v", err)
	}
}

func TestExtractReleaseBinaryFindsNestedEntry(t *testing.T) {
	payload := []byte("binary")
	archive := tarGzArchive(t, map[string][]byte{
		"README.md":          []byte("docs"),
		"dist/go/ballast-go": payload,
	})

	extracted, err := extractReleaseBinary(archive, "tar.gz", "ballast-go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(extracted, payload) {
		t.Fatalf("expected the nested binary payload, got %q", extracted)
	}
}

func TestExtractReleaseBinaryReportsMissingEntry(t *testing.T) {
	archive := tarGzArchive(t, map[string][]byte{"README.md": []byte("docs")})

	if _, err := extractReleaseBinary(archive, "tar.gz", "ballast-go"); err == nil {
		t.Fatal("expected a missing binary entry to fail")
	} else if !strings.Contains(err.Error(), "does not contain ballast-go") {
		t.Fatalf("expected a missing-entry error, got %v", err)
	}
}

func TestExtractReleaseBinaryReadsZipArchives(t *testing.T) {
	payload := []byte("windows binary")
	archive := zipArchive(t, map[string][]byte{"ballast-go.exe": payload})

	extracted, err := extractReleaseBinary(archive, "zip", "ballast-go.exe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(extracted, payload) {
		t.Fatalf("expected the zip payload, got %q", extracted)
	}
}

func TestExtractReleaseBinaryRejectsCorruptArchive(t *testing.T) {
	if _, err := extractReleaseBinary([]byte("not an archive"), "tar.gz", "ballast-go"); err == nil {
		t.Fatal("expected a corrupt archive to fail")
	} else if !strings.Contains(err.Error(), "open release archive") {
		t.Fatalf("expected an archive-open error, got %v", err)
	}
}

func TestExtractReleaseBinaryRejectsUnknownArchiveFormat(t *testing.T) {
	if _, err := extractReleaseBinary([]byte("payload"), "7z", "ballast-go"); err == nil {
		t.Fatal("expected an unknown archive format to fail")
	} else if !strings.Contains(err.Error(), "unsupported ballast-go archive format") {
		t.Fatalf("expected an unsupported-format error, got %v", err)
	}
}

// Reinstalling over an existing backend must replace it rather than fail, which
// is what `ballast doctor --fix` relies on for a version mismatch.
func TestWriteExecutableReplacesExistingBinary(t *testing.T) {
	directory := resolvedTempDir(t)
	destination := filepath.Join(directory, "bin", "ballast-go")

	if err := writeExecutable(destination, []byte("first")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := writeExecutable(destination, []byte("second")); err != nil {
		t.Fatalf("unexpected error on replace: %v", err)
	}

	content, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("read installed binary: %v", err)
	}
	if string(content) != "second" {
		t.Fatalf("expected the replacement payload, got %q", content)
	}

	entries, err := os.ReadDir(filepath.Dir(destination))
	if err != nil {
		t.Fatalf("read install directory: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected no temporary files left behind, got %d entries", len(entries))
	}
}
