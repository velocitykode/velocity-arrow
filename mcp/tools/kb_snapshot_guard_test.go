package tools

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestKBSnapshotMatchesPinnedVelocity fails the build when the embedded
// snapshot was built against a different velocity than go.mod pins. The
// release workflow regenerates the snapshot on every bump; this turns a
// forgotten or failed rebuild into a red test instead of a knowledge base
// that quietly describes an older framework.
func TestKBSnapshotMatchesPinnedVelocity(t *testing.T) {
	pinned := pinnedVelocityVersion(t)

	m, err := openKBStore(t).Manifest(context.Background())
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if m.VelocityVersion != pinned {
		t.Fatalf("snapshot built for velocity %s but go.mod pins %s; rebuild with cmd/ingest", m.VelocityVersion, pinned)
	}
}

// pinnedVelocityVersion reads the velocity requirement from the module's
// go.mod. The test binary's build info cannot be used: this package does not
// link velocity, so the dependency is absent from its module graph.
func pinnedVelocityVersion(t *testing.T) string {
	t.Helper()
	const module = "github.com/velocitykode/velocity"

	f, err := os.Open(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatalf("open go.mod: %v", err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) >= 2 && fields[0] == module && strings.HasPrefix(fields[1], "v") {
			return fields[1]
		}
		if len(fields) >= 3 && fields[0] == "replace" && fields[1] == module {
			t.Skip("velocity is replaced in go.mod; snapshot pin not comparable")
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	t.Fatalf("%s not required in go.mod", module)
	return ""
}
