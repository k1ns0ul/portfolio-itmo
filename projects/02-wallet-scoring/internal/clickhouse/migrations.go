package clickhouse

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

func Migrate(ctx context.Context, c *Client, dirFS fs.FS, dir string) error {
	names, err := listSQL(dirFS, dir)
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	if err := ensureRegistry(ctx, c); err != nil {
		return err
	}
	applied, err := appliedSet(ctx, c)
	if err != nil {
		return err
	}
	for _, name := range names {
		if applied[name] {
			continue
		}
		raw, err := fs.ReadFile(dirFS, dir+"/"+name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		if err := execScript(ctx, c, string(raw)); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if err := c.conn.Exec(ctx, "INSERT INTO wallets.schema_migrations (name) VALUES (?)", name); err != nil {
			return fmt.Errorf("mark %s: %w", name, err)
		}
	}
	return nil
}

func ensureRegistry(ctx context.Context, c *Client) error {
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
	return nil
}

func appliedSet(ctx context.Context, c *Client) (map[string]bool, error) {
	rows, err := c.conn.Query(ctx, "SELECT name FROM wallets.schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out[n] = true
	}
	return out, rows.Err()
}

func listSQL(fsys fs.FS, dir string) ([]string, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, nil
}

func execScript(ctx context.Context, c *Client, raw string) error {
	for _, stmt := range splitStatements(raw) {
		if err := c.conn.Exec(ctx, stmt); err != nil {
			return err
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
