package clickhouse

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

func Migrate(ctx context.Context, c *Client, dirFS fs.FS, dir string) error {
	if dir == "" {
		dir = "."
	}
	const ddl = `CREATE DATABASE IF NOT EXISTS wallets;
CREATE TABLE IF NOT EXISTS wallets.schema_migrations (
    name String,
    applied_at DateTime DEFAULT now()
) ENGINE = MergeTree() ORDER BY name;`
	for _, stmt := range splitStatements(ddl) {
		if err := c.conn.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("registry: %w", err)
		}
	}

	applied := map[string]bool{}
	rows, err := c.conn.Query(ctx, "SELECT name FROM wallets.schema_migrations")
	if err != nil {
		return err
	}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			rows.Close()
			return err
		}
		applied[n] = true
	}
	rows.Close()

	entries, err := fs.ReadDir(dirFS, dir)
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		if applied[name] {
			continue
		}
		raw, err := fs.ReadFile(dirFS, dir+"/"+name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		for _, stmt := range splitStatements(string(raw)) {
			if err := c.conn.Exec(ctx, stmt); err != nil {
				return fmt.Errorf("apply %s: %w", name, err)
			}
		}
		if err := c.conn.Exec(ctx,
			"INSERT INTO wallets.schema_migrations (name) VALUES (?)", name); err != nil {
			return fmt.Errorf("mark %s: %w", name, err)
		}
	}
	return nil
}

func splitStatements(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ";") {
		s := strings.TrimSpace(part)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}
