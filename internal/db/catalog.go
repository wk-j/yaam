package db

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wk-j/yaam/internal/config"
)

// Editable says how grid edits map a result back to its table.
type Editable struct {
	Schema        string    `json:"schema"`
	Table         string    `json:"table"`
	KeyColumns    []string  `json:"keyColumns"`
	ColumnSources []*string `json:"columnSources"` // per result column; nil = computed
}

// detectEditable returns non-nil when every table-sourced column comes from
// one table with a primary key and every key column is in the result.
func detectEditable(ctx context.Context, conn *pgxpool.Conn, cols []Column) *Editable {
	var table uint32
	present := map[uint16]bool{}
	for _, c := range cols {
		if c.tableOID == 0 {
			continue
		}
		if table != 0 && c.tableOID != table {
			return nil
		}
		table = c.tableOID
		present[c.attnum] = true
	}
	if table == 0 {
		return nil
	}
	ed := &Editable{KeyColumns: []string{}}
	err := conn.QueryRow(ctx, `SELECT n.nspname, c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.oid = $1 AND c.relkind IN ('r', 'p')`, table).Scan(&ed.Schema, &ed.Table)
	if err != nil {
		return nil
	}
	// indkey (int2vector) is subscripted from 0; -1 marks a non-key column.
	rows, err := conn.Query(ctx, `SELECT a.attnum, a.attname, coalesce(array_position(i.indkey::int2[], a.attnum), -1)
		FROM pg_attribute a LEFT JOIN pg_index i ON i.indrelid = a.attrelid AND i.indisprimary
		WHERE a.attrelid = $1 AND a.attnum > 0 AND NOT a.attisdropped`, table)
	if err != nil {
		return nil
	}
	names := map[uint16]string{}
	type key struct {
		pos  int
		name string
	}
	var keys []key
	for rows.Next() {
		var attnum int16
		var name string
		var pos int
		if rows.Scan(&attnum, &name, &pos) != nil {
			rows.Close()
			return nil
		}
		names[uint16(attnum)] = name
		if pos >= 0 {
			if !present[uint16(attnum)] {
				rows.Close()
				return nil
			}
			keys = append(keys, key{pos, name})
		}
	}
	rows.Close()
	if rows.Err() != nil || len(keys) == 0 {
		return nil
	}
	slices.SortFunc(keys, func(a, b key) int { return a.pos - b.pos })
	for _, k := range keys {
		ed.KeyColumns = append(ed.KeyColumns, k.name)
	}
	for _, c := range cols {
		var src *string
		if c.tableOID != 0 {
			if name, ok := names[c.attnum]; ok {
				src = &name
			}
		}
		ed.ColumnSources = append(ed.ColumnSources, src)
	}
	return ed
}

// Change is one grid edit: update the set columns, or delete the row, of
// the row with the given key. Values are PostgreSQL text; nil = NULL.
type Change struct {
	Op  string             `json:"op"`
	Key map[string]*string `json:"key"`
	Set map[string]*string `json:"set"`
}

func literal(v *string) string {
	if v == nil {
		return "NULL"
	}
	return "'" + strings.ReplaceAll(*v, "'", "''") + "'"
}

func ident(name string) string { return pgx.Identifier{name}.Sanitize() }

// sortedKeys gives map keys in order so the same edit always gives the
// same SQL.
func sortedKeys(m map[string]*string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// BuildEdits turns grid changes into one statement each, with quoted
// identifiers and quoted text literals.
func BuildEdits(schema, table string, changes []Change) ([]string, error) {
	if schema == "" || table == "" {
		return nil, errors.New("schema and table are required")
	}
	if len(changes) == 0 {
		return nil, errors.New("no changes")
	}
	target := pgx.Identifier{schema, table}.Sanitize()
	out := make([]string, 0, len(changes))
	for i, ch := range changes {
		if len(ch.Key) == 0 {
			return nil, fmt.Errorf("change %d has no key", i+1)
		}
		var where []string
		for _, k := range sortedKeys(ch.Key) {
			if ch.Key[k] == nil {
				return nil, fmt.Errorf("change %d: key column %s is null", i+1, k)
			}
			where = append(where, ident(k)+" = "+literal(ch.Key[k]))
		}
		cond := strings.Join(where, " AND ")
		switch ch.Op {
		case "update":
			if len(ch.Set) == 0 {
				return nil, fmt.Errorf("change %d sets no column", i+1)
			}
			var set []string
			for _, k := range sortedKeys(ch.Set) {
				set = append(set, ident(k)+" = "+literal(ch.Set[k]))
			}
			out = append(out, "UPDATE "+target+" SET "+strings.Join(set, ", ")+" WHERE "+cond+";")
		case "delete":
			out = append(out, "DELETE FROM "+target+" WHERE "+cond+";")
		default:
			return nil, fmt.Errorf("change %d: op must be update or delete", i+1)
		}
	}
	return out, nil
}

// EditRun is the outcome of one applied edit statement.
type EditRun struct {
	SQL        string
	DurationMs float64
	Rows       int64
}

// ApplyEdits runs stmts in one transaction and reports each statement.
func (m *Manager) ApplyEdits(ctx context.Context, name string, c config.Connection, stmts []string) (runs []EditRun, durationMs float64, err error) {
	_, conn, err := m.acquire(ctx, name, c)
	if err != nil {
		return nil, 0, err
	}
	defer release(ctx, conn)
	pc := conn.Conn().PgConn()
	start := time.Now()
	if err := simple(ctx, pc, "BEGIN"); err != nil {
		return nil, 0, err
	}
	for _, s := range stmts {
		t := time.Now()
		res, err := pc.Exec(ctx, s).ReadAll()
		if err != nil {
			cctx, cancel := cleanupCtx(ctx)
			simple(cctx, pc, "ROLLBACK")
			cancel()
			return nil, ms(time.Since(start)), errors.New(errorText(err))
		}
		runs = append(runs, EditRun{SQL: s, DurationMs: ms(time.Since(t)), Rows: res[0].CommandTag.RowsAffected()})
	}
	if err := simple(ctx, pc, "COMMIT"); err != nil {
		return nil, ms(time.Since(start)), errors.New(errorText(err))
	}
	return runs, ms(time.Since(start)), nil
}

// Table is a table, view or materialized view with its columns.
type Table struct {
	Schema        string        `json:"schema"`
	Name          string        `json:"name"`
	Kind          string        `json:"kind"` // table | view | materialized view
	Columns       []TableColumn `json:"columns"`
	HasPrimaryKey bool          `json:"hasPrimaryKey"`
}

type TableColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
	PK   bool   `json:"pk"`
}

const userSchemas = `n.nspname NOT IN ('pg_catalog', 'information_schema') AND n.nspname NOT LIKE 'pg\_toast%' AND n.nspname NOT LIKE 'pg\_temp\_%'`

// Schema lists the tables and views in non-system schemas.
func (m *Manager) Schema(ctx context.Context, name string, c config.Connection) ([]Table, error) {
	_, conn, err := m.acquire(ctx, name, c)
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	return tables(ctx, conn)
}

func tables(ctx context.Context, conn *pgxpool.Conn) ([]Table, error) {
	rows, err := conn.Query(ctx, `SELECT n.nspname, c.relname, c.relkind::text, a.attname, format_type(a.atttypid, a.atttypmod),
			coalesce(a.attnum = ANY(i.indkey::int2[]), false), i.indrelid IS NOT NULL
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum > 0 AND NOT a.attisdropped
		LEFT JOIN pg_index i ON i.indrelid = c.oid AND i.indisprimary
		WHERE c.relkind IN ('r', 'p', 'v', 'm') AND `+userSchemas+`
		ORDER BY n.nspname, c.relname, a.attnum`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Table{}
	for rows.Next() {
		var schema, rel, kind string
		var col TableColumn
		var hasPK bool
		if err := rows.Scan(&schema, &rel, &kind, &col.Name, &col.Type, &col.PK, &hasPK); err != nil {
			return nil, err
		}
		if n := len(out); n == 0 || out[n-1].Schema != schema || out[n-1].Name != rel {
			out = append(out, Table{Schema: schema, Name: rel, Kind: relKind(kind), Columns: []TableColumn{}, HasPrimaryKey: hasPK})
		}
		t := &out[len(out)-1]
		t.Columns = append(t.Columns, col)
	}
	return out, rows.Err()
}

func relKind(k string) string {
	switch k {
	case "v":
		return "view"
	case "m":
		return "materialized view"
	}
	return "table"
}

const maxMatches = 50

// Search finds tables and views (by name or column name), functions and
// indexes (by name or table name) whose names contain pattern, ignoring
// case; at most 50 lines.
func (m *Manager) Search(ctx context.Context, name string, c config.Connection, pattern string) ([]string, error) {
	_, conn, err := m.acquire(ctx, name, c)
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	pat := strings.ToLower(pattern)
	match := func(names ...string) bool {
		for _, n := range names {
			if strings.Contains(strings.ToLower(n), pat) {
				return true
			}
		}
		return false
	}
	out := []string{}

	ts, err := tables(ctx, conn)
	if err != nil {
		return nil, err
	}
	for _, t := range ts {
		names := []string{t.Schema + "." + t.Name}
		cols := make([]string, 0, len(t.Columns))
		for _, col := range t.Columns {
			names = append(names, col.Name)
			s := col.Name + " " + col.Type
			if col.PK {
				s += " pk"
			}
			cols = append(cols, s)
		}
		if match(names...) {
			out = append(out, fmt.Sprintf("%s %s.%s (%s)", t.Kind, t.Schema, t.Name, strings.Join(cols, ", ")))
			if len(out) == maxMatches {
				return out, nil
			}
		}
	}

	type lookup struct {
		sql    string
		format func(a, b, c, d string) (string, []string)
	}
	lookups := []lookup{
		{`SELECT n.nspname, p.proname, pg_get_function_identity_arguments(p.oid),
				CASE WHEN p.prokind = 'p' THEN '' ELSE coalesce(pg_get_function_result(p.oid), '') END
			FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
			WHERE p.prokind IN ('f', 'p') AND ` + userSchemas + ` ORDER BY 1, 2`,
			func(schema, fn, args, result string) (string, []string) {
				line := fmt.Sprintf("function %s.%s(%s)", schema, fn, args)
				if result != "" {
					line += " returns " + result
				}
				return line, []string{schema + "." + fn}
			}},
		{`SELECT n.nspname, ci.relname, ct.relname, pg_get_indexdef(i.indexrelid)
			FROM pg_index i
			JOIN pg_class ci ON ci.oid = i.indexrelid
			JOIN pg_class ct ON ct.oid = i.indrelid
			JOIN pg_namespace n ON n.oid = ci.relnamespace
			WHERE ` + userSchemas + ` ORDER BY 1, 2`,
			func(schema, index, table, def string) (string, []string) {
				return fmt.Sprintf("index %s.%s: %s", schema, index, def), []string{schema + "." + index, schema + "." + table}
			}},
	}
	for _, l := range lookups {
		rows, err := conn.Query(ctx, l.sql)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var a, b, c, d string
			if err := rows.Scan(&a, &b, &c, &d); err != nil {
				rows.Close()
				return nil, err
			}
			line, names := l.format(a, b, c, d)
			if match(names...) {
				out = append(out, line)
				if len(out) == maxMatches {
					rows.Close()
					return out, nil
				}
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}
