package main

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	defaultDataDir = "cc-flavors"
	defaultDBFile  = "events.sqlite"
)

var (
	pattern = regexp.MustCompile(`([A-Z][a-z]*ing)(?:\.\.\.|…)`)
)

type ingestConfig struct {
	rawLog string
	dbPath string
}

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stderr io.Writer) error {
	if len(args) == 0 {
		if err := printUsage(stderr); err != nil {
			return err
		}
		return nil
	}

	switch args[0] {
	case "ingest":
		cfg, err := parseIngestFlags(args[1:])
		if err != nil {
			return err
		}
		return runIngest(cfg, stdin)
	default:
		if err := printUsage(stderr); err != nil {
			return err
		}
		return nil
	}
}

func printUsage(w io.Writer) error {
	_, err := fmt.Fprintln(w, "usage: cc-flavors ingest [--db <path>]")
	return err
}

func parseIngestFlags(args []string) (ingestConfig, error) {
	fs := flag.NewFlagSet("ingest", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	cfg := ingestConfig{}
	fs.StringVar(&cfg.rawLog, "raw-log", "", "path to raw log (optional)")
	fs.StringVar(&cfg.dbPath, "db", "", "path to sqlite db")

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func runIngest(cfg ingestConfig, stdin io.Reader) (err error) {
	dbPath := cfg.dbPath
	if dbPath == "" {
		var err error
		dbPath, err = defaultDBPath()
		if err != nil {
			return err
		}
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
		return err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	if err := ensureSchema(db); err != nil {
		return err
	}

	stmt, err := db.Prepare(`
		INSERT INTO counts (word, count, created_at)
		VALUES (?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := stmt.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	counts := map[string]int{}
	scanner := bufio.NewScanner(stdin)
	for scanner.Scan() {
		addCounts(counts, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	if len(counts) == 0 {
		return nil
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	for word, count := range counts {
		if _, err := stmt.Exec(word, count, now); err != nil {
			return err
		}
	}
	return nil
}

func ensureSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS counts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			word TEXT NOT NULL,
			count INTEGER NOT NULL,
			created_at TEXT NOT NULL
		)
	`)
	return err
}

func addCounts(counts map[string]int, line string) {
	matches := pattern.FindAllStringSubmatch(line, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		counts[match[1]]++
	}
}

func defaultDBPath() (string, error) {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	if strings.TrimSpace(dataHome) == "" {
		return "", fmt.Errorf("XDG_DATA_HOME is empty")
	}
	return filepath.Join(dataHome, defaultDataDir, defaultDBFile), nil
}
