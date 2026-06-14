package tools

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/velocitykode/velocity"
	"github.com/velocitykode/velocity/orm"
)

// openManager builds a Velocity ORM manager from the project's .env config,
// honoring an optional database override.
func openManager(databaseOverride string) (*orm.Manager, error) {
	config := velocity.ConfigFromEnv()

	managerConfig := orm.ManagerConfig{
		Driver:          config.DB.Connection,
		Host:            config.DB.Host,
		Port:            config.DB.Port,
		Database:        config.DB.Database,
		Username:        config.DB.Username,
		Password:        config.DB.Password,
		Charset:         config.DB.Charset,
		SSLMode:         config.DB.SSLMode,
		MaxIdleConns:    config.DB.MaxIdleConns,
		MaxOpenConns:    config.DB.MaxOpenConns,
		ConnMaxLifetime: config.DB.ConnMaxLifetime,
	}

	if databaseOverride != "" {
		managerConfig.Database = databaseOverride
	}

	manager, err := orm.NewManager(managerConfig)
	if err != nil {
		return nil, fmt.Errorf("creating ORM manager: %w", err)
	}
	return manager, nil
}

// openDB opens a raw database connection using Velocity's config and ORM
// manager (used by db_query for arbitrary SQL).
func openDB(databaseOverride string) (*sql.DB, string, error) {
	manager, err := openManager(databaseOverride)
	if err != nil {
		return nil, "", err
	}
	return manager.DB(), manager.DriverName(), nil
}

// loadConfig returns the Velocity config from .env.
func loadConfig() velocity.Config {
	return velocity.ConfigFromEnv()
}

// secretKeys are .env keys that should be redacted in output.
var secretKeys = map[string]bool{
	"APP_KEY":               true,
	"DB_PASSWORD":           true,
	"REDIS_PASSWORD":        true,
	"JWT_SECRET":            true,
	"AWS_ACCESS_KEY_ID":     true,
	"AWS_SECRET_ACCESS_KEY": true,
	"CRYPTO_KEY":            true,
	"CRYPTO_OLD_KEYS":       true,
	"QUEUE_REDIS_PASSWORD":  true,
}

func isSecretKey(key string) bool {
	if secretKeys[key] {
		return true
	}
	upper := strings.ToUpper(key)
	return strings.Contains(upper, "SECRET") ||
		strings.Contains(upper, "PASSWORD") ||
		strings.Contains(upper, "TOKEN") && !strings.Contains(upper, "CSRF")
}
