package bootstrap

import (
	"fmt"
	"os"
)

func getDSN(user, password string) string {
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	dbname := getEnv("DB_NAME", "laguna_escondida")
	sslmode := getEnv("DB_SSLMODE", "disable")

	// TimeZone=UTC pins the connection's session timezone so timestamptz values
	// are read back and serialized in UTC (…Z) consistently, regardless of the
	// host's local timezone. (convertDSNToURL ignores this key, so the migrator
	// URL is unaffected.)
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=UTC",
		host, port, user, password, dbname, sslmode)
}

// getMigrationDSN falls back to the app credentials, so nothing changes until DB_MIGRATION_USER is set.
func getMigrationDSN() string {
	user := getEnv("DB_MIGRATION_USER", getEnv("DB_USER", "postgres"))
	password := getEnv("DB_MIGRATION_PASSWORD", getEnv("DB_PASSWORD", "postgres"))
	return getDSN(user, password)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
