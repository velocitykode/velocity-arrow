package tools

import "testing"

func TestIsAllowedQuery(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		allowed bool
	}{
		// Allowed
		{"select", "SELECT * FROM users", true},
		{"select lowercase", "select * from users", true},
		{"select with leading space", "  SELECT 1", true},
		{"show tables", "SHOW TABLES", true},
		{"explain", "EXPLAIN SELECT * FROM users", true},
		{"describe", "DESCRIBE users", true},
		{"desc", "DESC users", true},
		{"with select", "WITH cte AS (SELECT 1) SELECT * FROM cte", true},

		// Forbidden
		{"insert", "INSERT INTO users (name) VALUES ('test')", false},
		{"update", "UPDATE users SET name = 'test'", false},
		{"delete", "DELETE FROM users", false},
		{"drop", "DROP TABLE users", false},
		{"alter", "ALTER TABLE users ADD COLUMN age INT", false},
		{"create", "CREATE TABLE test (id INT)", false},
		{"truncate", "TRUNCATE TABLE users", false},
		{"grant", "GRANT ALL ON users TO admin", false},
		{"revoke", "REVOKE ALL ON users FROM admin", false},

		// WITH + mutation injection (security-critical)
		{"with delete injection", "WITH cte AS (DELETE FROM users RETURNING *) SELECT * FROM cte", false},
		{"with update injection", "WITH cte AS (UPDATE users SET admin=true RETURNING *) SELECT * FROM cte", false},
		{"with insert injection", "WITH cte AS (INSERT INTO users(name) VALUES('x') RETURNING *) SELECT * FROM cte", false},

		// Edge cases
		{"empty", "", false},
		{"just spaces", "   ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAllowedQuery(tt.query)
			if got != tt.allowed {
				t.Errorf("isAllowedQuery(%q) = %v, want %v", tt.query, got, tt.allowed)
			}
		})
	}
}
