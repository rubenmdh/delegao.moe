package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestMySQLSourceCertificatePin(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	defer server.Close()
	digest := sha256.Sum256(server.Certificate().Raw)
	pin := hex.EncodeToString(digest[:])
	for _, test := range []struct {
		name, pin string
		accepted  bool
	}{
		{"accepted", strings.ToUpper(pin), true},
		{"different certificate", strings.Repeat("00", 32), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg, err := mysqlSourceConfig("name-not-in-certificate.example", "source", "reader", "")
			if err != nil {
				t.Fatal(err)
			}
			if err := pinMySQLSourceCertificate(cfg, test.pin); err != nil {
				t.Fatal(err)
			}
			transport := &http.Transport{TLSClientConfig: cfg.TLS}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
			response, err := client.Get(server.URL)
			if response != nil {
				response.Body.Close()
			}
			if (err == nil) != test.accepted {
				t.Fatalf("TLS connection accepted=%v, want %v: %v", err == nil, test.accepted, err)
			}
		})
	}
	cfg, err := mysqlSourceConfig("source.example", "source", "reader", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"", "bad", strings.Repeat("zz", 32), strings.Repeat("00", 31)} {
		if pinMySQLSourceCertificate(cfg, invalid) == nil {
			t.Fatalf("invalid pin accepted: %q", invalid)
		}
	}
	if cfg.TLS.InsecureSkipVerify {
		t.Fatal("invalid pin changed default TLS verification")
	}
	// Accept the conventional colon-separated SHA-256 representation.
	colonPin := ""
	for i := 0; i < len(pin); i += 2 {
		if i != 0 {
			colonPin += ":"
		}
		colonPin += pin[i : i+2]
	}
	if err := pinMySQLSourceCertificate(cfg, colonPin); err != nil {
		t.Fatal(err)
	}
	if cfg.TLS.VerifyConnection(tls.ConnectionState{}) == nil {
		t.Fatal("missing certificate accepted")
	}
	for _, expiry := range []bool{false, true} {
		cert := *server.Certificate()
		if expiry {
			cert.NotAfter = time.Now().Add(-time.Hour)
		} else {
			cert.NotBefore = time.Now().Add(time.Hour)
		}
		if cfg.TLS.VerifyConnection(tls.ConnectionState{PeerCertificates: []*x509.Certificate{&cert}}) == nil {
			t.Fatal("certificate outside validity period accepted")
		}
	}
	cfg.TLS.RootCAs = x509.NewCertPool()
	if pinMySQLSourceCertificate(cfg, pin) == nil {
		t.Fatal("ambiguous CA and pin configuration accepted")
	}
	if pinMySQLSourceCertificate(nil, pin) == nil {
		t.Fatal("missing TLS configuration accepted")
	}
}
