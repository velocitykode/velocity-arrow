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
		{"QUEUE_SIGNING_KEY", true},
		{"STRIPE_API_KEY", true},
		{"SSH_PRIVATE_KEY", true},
		{"SOME_CREDENTIALS", true},
		{"SERVICE_PASSWD", true},
		{"CSRF_SECRET", true},

		{"APP_NAME", false},
		{"APP_ENV", false},
		{"DB_CONNECTION", false},
		{"DB_HOST", false},
		{"PORT", false},
		{"LOG_DRIVER", false},
		{"CSRF_TOKEN_LIFETIME", false}, // CSRF token is not secret
		{"CACHE_KEY_PREFIX", false},    // names a key, does not hold one
		{"AUTH_JWT_TTL", false},
		{"SESSION_HTTP_ONLY", false},
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
