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

func TestMariaDBDSNUsesTCPHostPort(t *testing.T) {
	cfg := mariaDBConfig{
		database: "app",
		hostname: "db.example.test",
		password: "secret",
		port:     "3307",
		user:     "app",
	}

	parsed, err := mysqlconfig.ParseDSN(cfg.dsn())
	if err != nil {
		t.Fatalf("ParseDSN returned error: %v", err)
	}
	if parsed.Net != "tcp" {
		t.Fatalf("Net = %q, want tcp", parsed.Net)
	}
	if parsed.Addr != "db.example.test:3307" {
		t.Fatalf("Addr = %q, want db.example.test:3307", parsed.Addr)
	}
}

func TestMariaDBDSNKeepsNeoShowcaseServiceHostnameOnTCP(t *testing.T) {
	cfg := mariaDBConfig{
		database: "h26s",
		hostname: "mariadb.ns-system.svc.cluster.local",
		password: "secret",
		port:     "3306",
		user:     "app",
	}

	parsed, err := mysqlconfig.ParseDSN(cfg.dsn())
	if err != nil {
		t.Fatalf("ParseDSN returned error: %v", err)
	}
	if parsed.Net != "tcp" {
		t.Fatalf("Net = %q, want tcp", parsed.Net)
	}
	if parsed.Addr != "mariadb.ns-system.svc.cluster.local:3306" {
		t.Fatalf("Addr = %q, want NeoShowcase service host and port", parsed.Addr)
	}
}

func TestMariaDBDSNUsesUnixSocketForAbsoluteHostname(t *testing.T) {
	cfg := mariaDBConfig{
		database: "app",
		hostname: "/cloudsql/h26s-06:asia-northeast1:client-db",
		password: "secret",
		port:     "3306",
		user:     "app",
	}

	parsed, err := mysqlconfig.ParseDSN(cfg.dsn())
	if err != nil {
		t.Fatalf("ParseDSN returned error: %v", err)
	}
	if parsed.Net != "unix" {
		t.Fatalf("Net = %q, want unix", parsed.Net)
	}
	if parsed.Addr != "/cloudsql/h26s-06:asia-northeast1:client-db" {
		t.Fatalf("Addr = %q, want Cloud SQL socket path", parsed.Addr)
	}
}
