package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var testBinary string

func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "cc-flavors-test")
	if err != nil {
		panic(err)
	}
	testBinary = filepath.Join(tmpDir, "cc-flavors")
	cmd := exec.Command("go", "build", "-o", testBinary, ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic(err)
	}
	code := m.Run()
	if err := os.RemoveAll(tmpDir); err != nil {
		panic(err)
	}
	os.Exit(code)
}

func runCLI(t *testing.T, input string, args ...string) (string, string) {
	t.Helper()

	cmd := exec.Command(testBinary, args...)
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("command failed: %v (stderr: %s)", err, stderr.String())
	}
	return stdout.String(), stderr.String()
}

func TestSummaryEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "events.sqlite")

	stdout, _ := runCLI(t, "", "summary", "--db", dbPath)
	if stdout != "No flavor texts found yet.\n" {
		t.Fatalf("unexpected output: %q", stdout)
	}
}

func TestIngestAndSummary(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "events.sqlite")

	input := strings.Join([]string{
		"Thinking... Moonwalking...",
		"Refactoring... Thinking...",
	}, "\n")
	_, _ = runCLI(t, input, "ingest", "--db", dbPath)

	stdout, _ := runCLI(t, "", "summary", "--db", dbPath)
	expected := strings.Join([]string{
		"Count  Flavor",
		"-----  ------",
		"    2  Thinking",
		"    1  Moonwalking",
		"    1  Refactoring",
		"",
	}, "\n")
	if stdout != expected {
		t.Fatalf("unexpected output:\n%s", stdout)
	}
}
