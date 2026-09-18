// Package kbsource decides which knowledge base arrow serves and where it
// comes from. The knowledge base is a build output of a velocity version, not
// a file arrow carries: for the version the current app compiles against it is
// taken from the local cache, else downloaded from that version's published
// release asset, else built on the spot from the module source already in the
// Go module cache. Nothing is baked into the binary.
package kbsource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Module is the framework module the knowledge base describes.
const Module = "github.com/velocitykode/velocity"

// DefaultBaseURL is where published snapshots live: one per framework
// release, attached to the velocity GitHub release for that tag.
const DefaultBaseURL = "https://github.com/velocitykode/velocity/releases/download"

// Pin is a resolved framework version with its source on disk.
type Pin struct {
	Version string    // vX.Y.Z
	Dir     string    // module source directory in the Go module cache
	Time    time.Time // module publish time; used as the reproducible build stamp
	Origin  string    // "app" when read from the app's go.mod, "latest" when nothing pins it
}

// Snapshot is a knowledge base file ready to open.
type Snapshot struct {
	Path    string
	Version string
	Source  string // "cache", "published" or "built"
}

// Options tune Ensure. Zero values mean: default published URL, no docs for a
// local build, silent.
type Options struct {
	BaseURL  string                                  // "" = DefaultBaseURL; "-" disables downloads
	DocsRoot string                                  // docs content tree for a local build; "" = symbols and rules only
	Logf     func(string, ...any)                    // progress sink, typically stderr
	Client   *http.Client                            // nil = 15s timeout client
	Build    func(context.Context, BuildInput) error // nil = Build
}

// ResolvePin reads the velocity version the module at dir compiles against.
// When dir is not inside a module requiring velocity it falls back to the
// latest published version, so arrow still answers outside an app tree.
func ResolvePin(ctx context.Context, dir string) (Pin, error) {
	if p, err := goListModule(ctx, dir, Module); err == nil {
		p.Origin = "app"
		return p, nil
	}
	p, err := Latest(ctx)
	if err != nil {
		return Pin{}, fmt.Errorf("kbsource: no velocity pin in %s and latest unavailable: %w", dir, err)
	}
	return p, nil
}

// Latest resolves the newest published velocity through the Go module proxy
// and downloads its source into the module cache.
func Latest(ctx context.Context) (Pin, error) {
	p, err := goListModule(ctx, "", Module+"@latest")
	if err != nil {
		return Pin{}, err
	}
	p.Origin = "latest"
	return p, nil
}

// goListModule runs `go list -m -json spec` and returns the resolved module.
// It downloads the module when needed so Dir is always populated.
func goListModule(ctx context.Context, dir, spec string) (Pin, error) {
	if _, err := exec.LookPath("go"); err != nil {
		return Pin{}, fmt.Errorf("kbsource: go toolchain not found: %w", err)
	}
	if err := runGo(ctx, dir, "mod", "download", "-json", spec); err != nil && !strings.Contains(spec, "@") {
		// Inside a module, `go mod download <module>` needs the module in
		// the build list. When it is not, `go list -m` below reports it.
		_ = err
	}
	out, err := outputGo(ctx, dir, "list", "-m", "-json", spec)
	if err != nil {
		return Pin{}, err
	}
	var m struct {
		Version string
		Dir     string
		Time    time.Time
		Replace *struct{ Path, Dir, Version string }
	}
	if err := json.Unmarshal(out, &m); err != nil {
		return Pin{}, fmt.Errorf("kbsource: parse go list: %w", err)
	}
	if m.Replace != nil {
		// A replaced module has no published version; describe what compiles.
		ver := m.Replace.Version
		if ver == "" {
			ver = "replace-" + filepath.Base(m.Replace.Dir)
		}
		return Pin{Version: ver, Dir: m.Replace.Dir, Time: time.Time{}}, nil
	}
	if m.Version == "" || m.Dir == "" {
		return Pin{}, fmt.Errorf("kbsource: %s not resolved (version %q, dir %q)", spec, m.Version, m.Dir)
	}
	return Pin{Version: m.Version, Dir: m.Dir, Time: m.Time}, nil
}

func goCmd(ctx context.Context, dir string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOWORK=off")
	return cmd
}

func runGo(ctx context.Context, dir string, args ...string) error {
	cmd := goCmd(ctx, dir, args...)
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	return cmd.Run()
}

func outputGo(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := goCmd(ctx, dir, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("kbsource: go %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// CacheDir is where snapshots live between runs: $ARROW_CACHE_DIR, else the
// user cache directory under arrow/kb. Safe to delete at any time.
func CacheDir() (string, error) {
	if d := os.Getenv("ARROW_CACHE_DIR"); d != "" {
		return d, nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("kbsource: user cache dir: %w", err)
	}
	return filepath.Join(base, "arrow", "kb"), nil
}

// Ensure returns a snapshot for pin: cached, else downloaded from the
// published release asset, else built from the module source.
func Ensure(ctx context.Context, pin Pin, o Options) (Snapshot, error) {
	logf := o.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}
	dir, err := CacheDir()
	if err != nil {
		return Snapshot{}, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Snapshot{}, fmt.Errorf("kbsource: create cache dir: %w", err)
	}
	path := filepath.Join(dir, pin.Version+".db")
	if st, err := os.Stat(path); err == nil && st.Size() > 0 {
		return Snapshot{Path: path, Version: pin.Version, Source: "cache"}, nil
	}

	if o.BaseURL != "-" {
		base := o.BaseURL
		if base == "" {
			base = DefaultBaseURL
		}
		url := strings.TrimRight(base, "/") + "/" + pin.Version + "/velocity-kb.db"
		if err := download(ctx, o.Client, url, path); err == nil {
			logf("kb: downloaded published snapshot for velocity %s", pin.Version)
			return Snapshot{Path: path, Version: pin.Version, Source: "published"}, nil
		} else {
			logf("kb: no published snapshot for velocity %s (%v); building from module source", pin.Version, err)
		}
	}

	build := o.Build
	if build == nil {
		build = Build
	}
	// Build into a private temp file and rename into place: two servers
	// starting at once (two editor sessions on the same app) each build
	// their own copy and the last rename wins with identical content.
	tmpf, err := os.CreateTemp(dir, pin.Version+".*.partial")
	if err != nil {
		return Snapshot{}, fmt.Errorf("kbsource: create build file: %w", err)
	}
	tmp := tmpf.Name()
	_ = tmpf.Close()
	_ = os.Remove(tmp) // the writer creates the database file itself
	in := BuildInput{VelocityDir: pin.Dir, Version: pin.Version, DocsRoot: o.DocsRoot, BuiltAt: pin.Time, Out: tmp}
	start := time.Now()
	if err := build(ctx, in); err != nil {
		_ = os.Remove(tmp)
		return Snapshot{}, fmt.Errorf("kbsource: build snapshot for %s: %w", pin.Version, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return Snapshot{}, fmt.Errorf("kbsource: install snapshot: %w", err)
	}
	logf("kb: built snapshot for velocity %s from %s in %s", pin.Version, pin.Dir, time.Since(start).Round(time.Millisecond))
	return Snapshot{Path: path, Version: pin.Version, Source: "built"}, nil
}

// download fetches url into path atomically. Any non-200 is an error so the
// caller falls through to a local build.
func download(ctx context.Context, client *http.Client, url, path string) error {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	tmp := path + ".download"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	n, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil || n == 0 {
		_ = os.Remove(tmp)
		return errors.Join(copyErr, closeErr, errIfZero(n))
	}
	return os.Rename(tmp, path)
}

func errIfZero(n int64) error {
	if n == 0 {
		return errors.New("empty body")
	}
	return nil
}

// Gap describes how far the app's pin is behind the latest release, in the
// terms a developer decides with: is it safe to bump, and what is new.
type Gap struct {
	Kind    string   // "none", "patch", "feature", "breaking" or "unknown"
	Latest  string   // latest version, "" when unknown
	Removed []string // symbols present at the pin and gone at latest
	Added   int      // symbols new at latest
}

// Classify compares two versions and their symbol sets. Semver gives the
// release type; the symbol diff says whether the bump can break this app.
func Classify(pin, latest string, pinSymbols, latestSymbols []string) Gap {
	if latest == "" {
		return Gap{Kind: "unknown"}
	}
	if pin == latest {
		return Gap{Kind: "none", Latest: latest}
	}
	g := Gap{Latest: latest}
	have := make(map[string]bool, len(latestSymbols))
	for _, s := range latestSymbols {
		have[s] = true
	}
	was := make(map[string]bool, len(pinSymbols))
	for _, s := range pinSymbols {
		was[s] = true
		if !have[s] {
			g.Removed = append(g.Removed, s)
		}
	}
	for _, s := range latestSymbols {
		if !was[s] {
			g.Added++
		}
	}
	switch {
	case len(g.Removed) > 0:
		g.Kind = "breaking"
	case g.Added > 0 || bumpKind(pin, latest) == "minor":
		g.Kind = "feature"
	default:
		g.Kind = "patch"
	}
	return g
}

// bumpKind reports "major", "minor" or "patch" for pin -> latest.
func bumpKind(pin, latest string) string {
	a, b := semver(pin), semver(latest)
	switch {
	case a[0] != b[0]:
		return "major"
	case a[1] != b[1]:
		return "minor"
	default:
		return "patch"
	}
}

func semver(v string) [3]int {
	var out [3]int
	parts := strings.SplitN(strings.TrimPrefix(v, "v"), ".", 3)
	for i := 0; i < len(parts) && i < 3; i++ {
		n, _ := strconv.Atoi(strings.SplitN(parts[i], "-", 2)[0])
		out[i] = n
	}
	return out
}

// Status is what the server knows about its knowledge base: what it serves,
// where it came from, and how the framework has moved since.
type Status struct {
	Pin      Pin
	Snapshot Snapshot
	Gap      Gap
}

// Nudge is the one line prepended to knowledge-base answers when the
// framework has moved past the app's pin. Empty when there is nothing to say.
func (s Status) Nudge() string {
	g := s.Gap
	switch g.Kind {
	case "patch":
		return fmt.Sprintf("velocity %s is available (patch over your %s): safe to bump, nothing to change.", g.Latest, s.Pin.Version)
	case "feature":
		return fmt.Sprintf("velocity %s is available (you pin %s): %d new symbols, none removed. New APIs are not in this knowledge base until you bump.", g.Latest, s.Pin.Version, g.Added)
	case "breaking":
		list := g.Removed
		more := ""
		if len(list) > 5 {
			more = fmt.Sprintf(" and %d more", len(list)-5)
			list = list[:5]
		}
		return fmt.Sprintf("velocity %s is available (you pin %s) and removes %d symbols: %s%s. Plan a migration before bumping.", g.Latest, s.Pin.Version, len(g.Removed), strings.Join(list, ", "), more)
	}
	return ""
}
