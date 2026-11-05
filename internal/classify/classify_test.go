package classify

import (
	"slices"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name string
		sql  string
		want []Kind
	}{
		{"plain select", "SELECT * FROM orders WHERE id = 1", []Kind{Read}},
		{"select for update", "SELECT * FROM orders FOR UPDATE", []Kind{Write}},
		{"locking clause in subquery", "SELECT * FROM (SELECT * FROM orders FOR SHARE) o", []Kind{Write}},
		{"with delete select", "WITH d AS (DELETE FROM orders RETURNING *) SELECT * FROM d", []Kind{Write}},
		{"with insert select", "WITH i AS (INSERT INTO t VALUES (1) RETURNING *) SELECT * FROM i", []Kind{Write}},
		{"with update select", "WITH u AS (UPDATE t SET a = 1 RETURNING *) SELECT * FROM u", []Kind{Write}},
		{"read-only with", "WITH r AS (SELECT 1 AS a) SELECT a FROM r", []Kind{Read}},
		{"select into", "SELECT * INTO new_orders FROM orders", []Kind{Write}},
		{"show", "SHOW search_path", []Kind{Read}},
		{"explain", "EXPLAIN SELECT * FROM orders", []Kind{Read}},
		{"explain options without analyze", "EXPLAIN (COSTS false) SELECT 1", []Kind{Read}},
		{"explain analyze", "EXPLAIN ANALYZE SELECT * FROM orders", []Kind{Write}},
		{"explain (analyze)", "EXPLAIN (ANALYZE, BUFFERS) DELETE FROM orders", []Kind{Write}},
		{"function call accepted gap", "SELECT archive()", []Kind{Read}},
		{"call", "CALL do_things()", []Kind{Write}},
		{"do", "DO $$ BEGIN PERFORM 1; END $$", []Kind{Write}},
		{"copy", "COPY orders TO STDOUT", []Kind{Write}},
		{"set", "SET search_path = public", []Kind{Write}},
		{"lock", "LOCK TABLE orders", []Kind{Write}},
		{"ddl", "CREATE TABLE t (id int)", []Kind{Write}},
		{"insert", "INSERT INTO t VALUES (1)", []Kind{Write}},
		{"begin", "BEGIN", []Kind{TxControl}},
		{"commit", "COMMIT", []Kind{TxControl}},
		{"end", "END", []Kind{TxControl}},
		{"rollback", "ROLLBACK", []Kind{TxControl}},
		{"abort", "ABORT", []Kind{TxControl}},
		{"start transaction", "START TRANSACTION", []Kind{TxControl}},
		{"prepare transaction", "PREPARE TRANSACTION 'x'", []Kind{TxControl}},
		{"savepoint", "SAVEPOINT s", []Kind{Write}},
		{"release", "RELEASE SAVEPOINT s", []Kind{Write}},
		{"rollback to", "ROLLBACK TO SAVEPOINT s", []Kind{Write}},
		{"multi-statement mix", "SELECT 1; DELETE FROM orders WHERE id = 3;\nSHOW work_mem; COMMIT", []Kind{Read, Write, Read, TxControl}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stmts, err := Parse(c.sql)
			if err != nil {
				t.Fatalf("Parse(%q): %v", c.sql, err)
			}
			var got []Kind
			for _, s := range stmts {
				got = append(got, s.Kind)
			}
			if !slices.Equal(got, c.want) {
				t.Fatalf("Parse(%q) kinds = %v, want %v", c.sql, got, c.want)
			}
		})
	}
}

func TestParseStatementText(t *testing.T) {
	stmts, err := Parse("  SELECT 1;\n  DELETE FROM orders WHERE note = 'a;b'  ;SELECT 2")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"SELECT 1", "DELETE FROM orders WHERE note = 'a;b'", "SELECT 2"}
	var got []string
	for _, s := range stmts {
		got = append(got, s.Text)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("texts = %q, want %q", got, want)
	}
}

func TestParseSyntaxError(t *testing.T) {
	stmts, err := Parse("SELEC 1")
	if err == nil {
		t.Fatalf("Parse succeeded with %v, want a syntax error", stmts)
	}
	if want := `syntax error at or near "SELEC"`; err.Error() != want {
		t.Fatalf("error = %q, want %q", err, want)
	}
}
