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
	"runtime/debug"
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

type exportConfig struct {
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
	case "--version", "-V", "version":
		return printVersion(os.Stdout)
	case "ingest":
		cfg, err := parseIngestFlags(args[1:])
		if err != nil {
			return err
		}
		return runIngest(cfg, stdin)
	case "summary":
		cfg, err := parseSummaryFlags(args[1:])
		if err != nil {
			return err
		}
		return runSummary(cfg, os.Stdout)
	default:
		if err := printUsage(stderr); err != nil {
			return err
		}
		return nil
	}
}

func printUsage(w io.Writer) error {
	usage := `usage: cc-flavors <command> [options]

commands:
  ingest  read from stdin and store counts
  summary  print aggregated counts
  version  print version

options:
  --db <path>  sqlite db path (default: $XDG_DATA_HOME/cc-flavors/events.sqlite)`
	_, err := fmt.Fprintln(w, usage)
	return err
}

func printVersion(w io.Writer) error {
	version := buildVersion()
	_, err := fmt.Fprintln(w, version)
	return err
}

func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return "0.0.0"
	}
	return strings.TrimPrefix(info.Main.Version, "v")
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

func parseSummaryFlags(args []string) (exportConfig, error) {
	fs := flag.NewFlagSet("summary", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	cfg := exportConfig{}
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

func runSummary(cfg exportConfig, w io.Writer) error {
	dbPath := cfg.dbPath
	if dbPath == "" {
		var err error
		dbPath, err = defaultDBPath()
		if err != nil {
			return err
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = db.Close()
	}()

	if err := ensureSchema(db); err != nil {
		return err
	}

	rows, err := db.Query(`
		SELECT word, SUM(count) AS total
		FROM counts
		GROUP BY word
		ORDER BY total DESC, word ASC
	`)
	if err != nil {
		return err
	}
	defer func() {
		_ = rows.Close()
	}()

	hasRows := false
	for rows.Next() {
		var word string
		var total int
		if err := rows.Scan(&word, &total); err != nil {
			return err
		}
		if !hasRows {
			if _, err := fmt.Fprintln(w, "Count  Flavor"); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(w, "-----  ------"); err != nil {
				return err
			}
			hasRows = true
		}
		if _, err := fmt.Fprintf(w, "%5d  %s\n", total, word); err != nil {
			return err
		}
	}
	if !hasRows {
		_, err := fmt.Fprintln(w, "No flavor texts found yet.")
		return err
	}
	if err := rows.Err(); err != nil {
		return err
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
