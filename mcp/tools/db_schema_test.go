package tools

import (
	"context"
	"strings"
	"testing"

	ormtesting "github.com/velocitykode/velocity/orm/testing"
)

func boolPtr(v bool) *bool { return &v }

func TestResolveDetail(t *testing.T) {
	tests := []struct {
		name       string
		detail     string
		summary    *bool
		tableCount int
		want       string
		wantErr    bool
	}{
		// Explicit detail always wins.
		{"explicit columns", detailColumns, nil, 1, detailColumns, false},
		{"explicit full", detailFull, nil, 10, detailFull, false},
		{"explicit columns beats summary=false", detailColumns, boolPtr(false), 1, detailColumns, false},
		{"explicit full beats summary=true", detailFull, boolPtr(true), 10, detailFull, false},

		// Legacy summary flag next.
		{"summary true maps to columns", "", boolPtr(true), 1, detailColumns, false},
		{"summary false maps to full", "", boolPtr(false), 10, detailFull, false},

		// Defaults: single table -> full, listing -> columns.
		{"single table defaults to full", "", nil, 1, detailFull, false},
		{"two tables default to columns", "", nil, 2, detailColumns, false},
		{"many tables default to columns", "", nil, 40, detailColumns, false},

		// Invalid values are rejected, not silently defaulted.
		{"invalid detail", "verbose", nil, 1, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveDetail(tt.detail, tt.summary, tt.tableCount)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("resolveDetail(%q) expected error, got %q", tt.detail, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveDetail(%q) unexpected error: %v", tt.detail, err)
			}
			if got != tt.want {
				t.Errorf("resolveDetail(%q, %v, %d) = %q, want %q", tt.detail, tt.summary, tt.tableCount, got, tt.want)
			}
		})
	}
}

func TestQuoteIdent(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain", "users", `"users"`},
		{"embedded quote doubled", `us"ers`, `"us""ers"`},
		{"empty", "", `""`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := quoteIdent(tt.input); got != tt.want {
				t.Errorf("quoteIdent(%q) = %s, want %s", tt.input, got, tt.want)
			}
		})
	}
}

func TestWriteBulletSection(t *testing.T) {
	tests := []struct {
		name    string
		heading string
		lines   []string
		want    string
	}{
		{"empty is explicit", "Indexes", nil, "Indexes:\n- (none)\n\n"},
		{"lines listed", "Constraints", []string{"a", "b"}, "Constraints:\n- a\n- b\n\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			writeBulletSection(&b, tt.heading, tt.lines)
			if b.String() != tt.want {
				t.Errorf("writeBulletSection(%q, %v) = %q, want %q", tt.heading, tt.lines, b.String(), tt.want)
			}
		})
	}
}

func TestWriteIndexesAndConstraints_UnsupportedDriver(t *testing.T) {
	var b strings.Builder
	writeIndexesAndConstraints(context.Background(), &b, nil, "oracle", "users", nil)

	text := b.String()
	if !strings.Contains(text, `Indexes: unavailable for driver "oracle"`) {
		t.Errorf("expected explicit indexes-unavailable line, got: %s", text)
	}
	if !strings.Contains(text, `Constraints: unavailable for driver "oracle"`) {
		t.Errorf("expected explicit constraints-unavailable line, got: %s", text)
	}
}

// seedIndexedSchema creates a users/posts pair carrying every SQLite index and
// constraint shape the tool reports: PK, UNIQUE constraint, plain / unique /
// partial CREATE INDEX, and an FK.
func seedIndexedSchema(t *testing.T, exec func(query string, args ...any) error) {
	t.Helper()
	statements := []string{
		`CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			email TEXT NOT NULL,
			name TEXT,
			deleted_at TEXT,
			UNIQUE (email)
		)`,
		`CREATE INDEX idx_users_name ON users(name)`,
		`CREATE UNIQUE INDEX idx_users_active_email ON users(email) WHERE deleted_at IS NULL`,
		`CREATE TABLE posts (
			id INTEGER PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id),
			title TEXT
		)`,
	}
	for _, stmt := range statements {
		if err := exec(stmt); err != nil {
			t.Fatalf("seeding schema: %v", err)
		}
	}
}

func TestSQLiteIndexesAndConstraints(t *testing.T) {
	manager := newTestManager(t)
	tc := ormtesting.Setup(t, manager)
	db := tc.DB()
	seedIndexedSchema(t, func(query string, args ...any) error {
		_, err := db.Exec(query, args...)
		return err
	})

	ctx := context.Background()

	cols, err := manager.DescribeTable(ctx, "users")
	if err != nil {
		t.Fatalf("DescribeTable(users): %v", err)
	}
	indexes, constraints, err := sqliteIndexesAndConstraints(ctx, manager, "users", cols)
	if err != nil {
		t.Fatalf("sqliteIndexesAndConstraints(users): %v", err)
	}

	joinedIdx := strings.Join(indexes, "\n")
	if !strings.Contains(joinedIdx, "idx_users_name: (name)") {
		t.Errorf("indexes should list idx_users_name with its column, got:\n%s", joinedIdx)
	}
	if !strings.Contains(joinedIdx, "idx_users_active_email: (email) UNIQUE PARTIAL") {
		t.Errorf("indexes should flag unique partial index, got:\n%s", joinedIdx)
	}

	joinedCons := strings.Join(constraints, "\n")
	if !strings.Contains(joinedCons, "PRIMARY KEY (id)") {
		t.Errorf("constraints should include the primary key, got:\n%s", joinedCons)
	}
	if !strings.Contains(joinedCons, "UNIQUE (email)") {
		t.Errorf("constraints should include the unique constraint, got:\n%s", joinedCons)
	}

	// FK lives on posts.
	postCols, err := manager.DescribeTable(ctx, "posts")
	if err != nil {
		t.Fatalf("DescribeTable(posts): %v", err)
	}
	_, postCons, err := sqliteIndexesAndConstraints(ctx, manager, "posts", postCols)
	if err != nil {
		t.Fatalf("sqliteIndexesAndConstraints(posts): %v", err)
	}
	joinedPost := strings.Join(postCons, "\n")
	if !strings.Contains(joinedPost, "FOREIGN KEY (user_id) REFERENCES users(id)") {
		t.Errorf("posts constraints should include the FK, got:\n%s", joinedPost)
	}
}

func TestWriteTableSection_DetailLevels(t *testing.T) {
	manager := newTestManager(t)
	tc := ormtesting.Setup(t, manager)
	db := tc.DB()
	seedIndexedSchema(t, func(query string, args ...any) error {
		_, err := db.Exec(query, args...)
		return err
	})

	ctx := context.Background()

	t.Run("columns detail stays lean", func(t *testing.T) {
		var b strings.Builder
		writeTableSection(ctx, &b, manager, "users", detailColumns)
		text := b.String()
		if !strings.Contains(text, "## users") {
			t.Errorf("should contain table heading, got:\n%s", text)
		}
		if !strings.Contains(text, "- email") {
			t.Errorf("should list email column, got:\n%s", text)
		}
		if strings.Contains(text, "Indexes:") || strings.Contains(text, "Constraints:") {
			t.Errorf("columns detail must not include indexes/constraints, got:\n%s", text)
		}
	})

	t.Run("full detail includes indexes and constraints", func(t *testing.T) {
		var b strings.Builder
		writeTableSection(ctx, &b, manager, "users", detailFull)
		text := b.String()
		if !strings.Contains(text, "| Column | Type | Nullable | Default | Key |") {
			t.Errorf("should contain column table, got:\n%s", text)
		}
		if !strings.Contains(text, "Indexes:") {
			t.Errorf("should contain Indexes section, got:\n%s", text)
		}
		if !strings.Contains(text, "idx_users_name: (name)") {
			t.Errorf("should list idx_users_name, got:\n%s", text)
		}
		if !strings.Contains(text, "Constraints:") {
			t.Errorf("should contain Constraints section, got:\n%s", text)
		}
		if !strings.Contains(text, "PRIMARY KEY (id)") {
			t.Errorf("should list primary key constraint, got:\n%s", text)
		}
	})

	t.Run("full detail on table without indexes says none", func(t *testing.T) {
		if _, err := db.Exec("CREATE TABLE bare (note TEXT)"); err != nil {
			t.Fatalf("creating bare table: %v", err)
		}
		var b strings.Builder
		writeTableSection(ctx, &b, manager, "bare", detailFull)
		text := b.String()
		if !strings.Contains(text, "Indexes:\n- (none)") {
			t.Errorf("empty index section must be explicit, got:\n%s", text)
		}
	})
}
