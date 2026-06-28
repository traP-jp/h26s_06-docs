package main

import (
	"testing"

	mysqlconfig "github.com/go-sql-driver/mysql"
)

func TestMariaDBDSNAllowsNativePasswordAuthentication(t *testing.T) {
	cfg := mariaDBConfig{
		database: "app",
		hostname: "db.example.test",
		password: "secret",
		port:     "3306",
		user:     "app",
	}

	parsed, err := mysqlconfig.ParseDSN(cfg.dsn())
	if err != nil {
		t.Fatalf("ParseDSN returned error: %v", err)
	}
	if !parsed.AllowNativePasswords {
		t.Fatal("AllowNativePasswords = false, want true")
	}
}
