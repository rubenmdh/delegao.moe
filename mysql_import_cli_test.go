package main

import (
	"context"
	"crypto/tls"
	"os"
	"path/filepath"
	"testing"
)

func TestMySQLSourceRequiresVerifiedTLS(t *testing.T) {
	for _, test := range []struct{ host, address, server string }{{"db.example.com", "db.example.com:3306", "db.example.com"}, {"db.example.com:3307", "db.example.com:3307", "db.example.com"}, {"::1", "[::1]:3306", "::1"}} {
		cfg, err := mysqlSourceConfig(test.host, "source", "reader", "")
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Addr != test.address || cfg.TLS == nil || cfg.TLS.InsecureSkipVerify || cfg.TLS.MinVersion != tls.VersionTLS12 || cfg.TLS.ServerName != test.server || cfg.Passwd != "" {
			t.Fatal("source does not require verified TLS")
		}
	}
	for _, host := range []string{"", "https://example.com", "example.com:0", "example.com:65536", "user@example.com", "bad host"} {
		if _, err := mysqlSourceConfig(host, "source", "reader", ""); err == nil {
			t.Fatalf("bad source host: %s", host)
		}
	}
	ca := filepath.Join(t.TempDir(), "invalid.pem")
	if err := os.WriteFile(ca, []byte("not a CA certificate"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := mysqlSourceConfig("db.example.com", "source", "reader", ca); err == nil {
		t.Fatal("invalid CA accepted")
	}
	if err := runMySQLImport(context.Background(), &Store{dialect: "mysql"}, []string{"host", "db", "user"}); err == nil {
		t.Fatal("import accepted non-SQLite destination")
	}
}
