package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/go-sql-driver/mysql"
	"golang.org/x/term"
)

func mysqlSourceConfig(host, database, username, caPath string) (*mysql.Config, error) {
	if host == "" || database == "" || username == "" {
		return nil, errors.New("source host, database and username are required")
	}
	address := host
	if _, _, err := net.SplitHostPort(address); err != nil {
		address = net.JoinHostPort(strings.Trim(host, "[]"), "3306")
	}
	hostname, port, err := net.SplitHostPort(address)
	if err != nil || hostname == "" || strings.ContainsAny(hostname, " /\\\t\r\n@?#") {
		return nil, errors.New("invalid MySQL source host")
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return nil, errors.New("invalid MySQL source port")
	}
	cfg := mysql.NewConfig()
	cfg.Net = "tcp"
	cfg.Addr = address
	cfg.DBName = database
	cfg.User = username
	cfg.TLS = &tls.Config{MinVersion: tls.VersionTLS12, ServerName: hostname}
	if caPath != "" {
		info, err := os.Stat(caPath)
		if err != nil {
			return nil, fmt.Errorf("read MYSQL_SOURCE_CA: %w", err)
		}
		if info.Size() > 1<<20 {
			return nil, errors.New("MYSQL_SOURCE_CA must be at most 1 MiB")
		}
		pem, err := os.ReadFile(caPath)
		if err != nil {
			return nil, fmt.Errorf("read MYSQL_SOURCE_CA: %w", err)
		}
		roots, err := x509.SystemCertPool()
		if err != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, errors.New("MYSQL_SOURCE_CA does not contain a valid PEM certificate")
		}
		cfg.TLS.RootCAs = roots
	}
	return cfg, nil
}

func runMySQLImport(ctx context.Context, target *Store, args []string) error {
	if len(args) != 3 {
		return errors.New("usage: delegao import-mysql HOST DATABASE USERNAME (password is prompted)")
	}
	if target.dialect != "sqlite" {
		return errors.New("import-mysql requires DATABASE_DRIVER=sqlite and an empty SQLite target")
	}
	cfg, err := mysqlSourceConfig(args[0], args[1], args[2], os.Getenv("MYSQL_SOURCE_CA"))
	if err != nil {
		return err
	}
	fmt.Fprint(os.Stderr, "MySQL source password (hidden): ")
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return err
	}
	cfg.Passwd = string(password)
	clear(password)
	source, err := openMySQLConfig(cfg)
	if err != nil {
		return fmt.Errorf("cannot connect to MySQL source with verified TLS: %w", err)
	}
	defer source.db.Close()
	if err := target.migrate(ctx); err != nil {
		return err
	}
	summary, err := importMySQL(ctx, source, target)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Imported %d posts, %d guestbook entries and %d users. The MySQL source was not modified.\n", summary.Posts, summary.Guests, summary.Users)
	return nil
}
