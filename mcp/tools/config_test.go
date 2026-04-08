package tools

import "testing"

func TestIsSecretKey(t *testing.T) {
	tests := []struct {
		key    string
		secret bool
	}{
		{"APP_KEY", true},
		{"DB_PASSWORD", true},
		{"JWT_SECRET", true},
		{"AWS_SECRET_ACCESS_KEY", true},
		{"CRYPTO_KEY", true},
		{"REDIS_PASSWORD", true},
		{"SOME_PASSWORD_FIELD", true},
		{"MY_SECRET_VALUE", true},

		{"APP_NAME", false},
		{"APP_ENV", false},
		{"DB_CONNECTION", false},
		{"DB_HOST", false},
		{"PORT", false},
		{"LOG_DRIVER", false},
		{"CSRF_TOKEN_LIFETIME", false}, // CSRF token is not secret
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := isSecretKey(tt.key)
			if got != tt.secret {
				t.Errorf("isSecretKey(%q) = %v, want %v", tt.key, got, tt.secret)
			}
		})
	}
}
