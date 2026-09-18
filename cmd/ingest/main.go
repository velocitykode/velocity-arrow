// Command ingest builds a knowledge-base snapshot for one velocity version: it
// parses exact symbols from the framework source, loads the curated guard
// rules, adds the published docs pages, and writes a SQLite file. The release
// pipeline runs it per framework release and publishes the file as that
// release's asset; arrow downloads it on demand (see internal/kbsource).
// Nothing is committed or embedded.
//
// Usage:
//
//	go run ./cmd/ingest -velocity "$(go list -m -f '{{.Dir}}' github.com/velocitykode/velocity)" \
//	    -version v0.81.1 -docs ~/code/velocity-docs/content/docs -out velocity-kb.db
//
// -docs points at the Hugo content tree of the published docs site (vel.build);
// omit it to build symbols and rules only. SOURCE_DATE_EPOCH, when set, is the
// manifest timestamp so identical inputs give an identical file.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/velocitykode/velocity-arrow/internal/kbsource"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ingest:", err)
		os.Exit(1)
	}
}

func run() error {
	root := flag.String("velocity", os.Getenv("VELOCITY_SRC"), "path to the velocity source tree (default $VELOCITY_SRC)")
	version := flag.String("version", "dev", "velocity version stamp for entries and manifest")
	out := flag.String("out", "velocity-kb.db", "output snapshot path")
	docsRoot := flag.String("docs", "", "path to the published docs content tree (e.g. ~/code/velocity-docs/content/docs); empty skips doc pages")
	flag.Parse()

	if *root == "" {
		return fmt.Errorf("-velocity is required (or set VELOCITY_SRC)")
	}
	builtAt, err := buildTimestamp()
	if err != nil {
		return err
	}
	if *docsRoot == "" {
		fmt.Fprintln(os.Stderr, "ingest: no -docs path; snapshot will have no documentation pages")
	}

	in := kbsource.BuildInput{VelocityDir: *root, Version: *version, DocsRoot: *docsRoot, BuiltAt: builtAt, Out: *out}
	if err := kbsource.Build(context.Background(), in); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "ingest: wrote %s (velocity %s)\n", *out, *version)
	return nil
}

// buildTimestamp honours SOURCE_DATE_EPOCH (the reproducible-builds convention)
// so two ingests of the same inputs produce an identical snapshot. Unset, it
// falls back to now.
func buildTimestamp() (time.Time, error) {
	raw := os.Getenv("SOURCE_DATE_EPOCH")
	if raw == "" {
		return time.Now().UTC(), nil
	}
	secs, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("SOURCE_DATE_EPOCH %q: %w", raw, err)
	}
	return time.Unix(secs, 0).UTC(), nil
}
