package db

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/wk-j/yaam/internal/config"
)

// pgConnection reads YAAM_TEST_PG ("host:port:database:role", trust auth) and
// skips the test when it is unset, so the suite needs no database by default.
func pgConnection(t *testing.T) config.Connection {
	t.Helper()
	v := os.Getenv("YAAM_TEST_PG")
	if v == "" {
		t.Skip("YAAM_TEST_PG not set")
	}
	parts := strings.Split(v, ":")
	if len(parts) != 4 {
		t.Fatalf("YAAM_TEST_PG = %q, want host:port:database:role", v)
	}
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	return config.Connection{Host: parts[0], Port: port, Database: parts[2], Role: parts[3], SSLMode: "disable", AgentTimeoutSeconds: 30}
}

// A Request ends with DISCARD ALL, which drops the session's prepared
// statements; yaam's own catalog queries on the same conn must still work.
func TestCatalogQueriesAfterRequest(t *testing.T) {
	c := pgConnection(t)
	m := NewManager(func(string) (string, error) { return "", nil }, func(string, error) {})
	defer m.Close()
	ctx := context.Background()
	for i := range 3 {
		if _, err := m.Search(ctx, "test", c, "pg_"); err != nil {
			t.Fatalf("round %d: Search: %v", i, err)
		}
		if _, err := m.Schema(ctx, "test", c); err != nil {
			t.Fatalf("round %d: Schema: %v", i, err)
		}
		if res, _ := m.RunAgent(ctx, "test", c, "SELECT 1"); res.Error != "" {
			t.Fatalf("round %d: RunAgent: %s", i, res.Error)
		}
	}
}
