package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestRunVersionSkipsLanguageDetection(t *testing.T) {
	t.Run("plain version flag", func(t *testing.T) {
		output := captureStdout(t, func() {
			exitCode := run([]string{"--version"})
			if exitCode != 0 {
				t.Fatalf("expected exit code 0, got %d", exitCode)
			}
		})

		if got := strings.TrimSpace(output); got != version {
			t.Fatalf("expected version output %q, got %q", version, got)
		}
	})

	t.Run("version with explicit language", func(t *testing.T) {
		output := captureStdout(t, func() {
			exitCode := run([]string{"--language", "go", "--version"})
			if exitCode != 0 {
				t.Fatalf("expected exit code 0, got %d", exitCode)
			}
		})

		if got := strings.TrimSpace(output); got != version {
			t.Fatalf("expected version output %q, got %q", version, got)
		}
	})
}

func TestRunWithoutArgsPrintsUsage(t *testing.T) {
	output := captureStdout(t, func() {
		exitCode := run(nil)
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", exitCode)
		}
	})

	if !strings.Contains(output, "Usage:\n  ballast [flags] <command> [command flags]") {
		t.Fatalf("expected usage output, got %q", output)
	}
	if !strings.Contains(output, "Commands:") {
		t.Fatalf("expected commands section, got %q", output)
	}
	if !strings.Contains(output, "Flags:") {
		t.Fatalf("expected flags section, got %q", output)
	}
	if strings.Contains(output, "Could not detect repository language") {
		t.Fatalf("expected no language detection error, got %q", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}

	os.Stdout = writer
	t.Cleanup(func() {
		os.Stdout = originalStdout
	})

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close reader: %v", err)
	}

	return buf.String()
}
