// Package db runs SQL on Connections with pgx: Agent Requests as one
// transaction each (ADR 0005), the User's editor runs like psql, grid edits,
// and yaam's own catalog lookups.
package db

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wk-j/yaam/internal/config"
)

// Column is one result column.
type Column struct {
	Name string `json:"name"`
	Type string `json:"type"`

	typeOID  uint32
	tableOID uint32
	attnum   uint16
}

// Statement is the result of one statement of a run.
type Statement struct {
	Command  string      `json:"command"`
	Columns  []Column    `json:"columns"`
	Rows     [][]*string `json:"rows"` // PostgreSQL text format; nil = NULL
	RowCount int64       `json:"rowCount"`
}

// Result is what a run returns: every statement that completed, and the
// PostgreSQL error of the statement that failed, if any.
type Result struct {
	Statements []Statement `json:"statements"`
	DurationMs float64     `json:"durationMs"`
	Error      string      `json:"error"`
	Notices    []string    `json:"notices"`
}

// TotalRows sums the row counts of all statements.
func (r *Result) TotalRows() int64 {
	var n int64
	for _, s := range r.Statements {
		n += s.RowCount
	}
	return n
}

func newResult() Result {
	return Result{Statements: []Statement{}, Notices: []string{}}
}

// Manager keeps one pgxpool per Connection, built lazily from the
// Connection's config and Keychain password.
type Manager struct {
	password func(name string) (string, error)
	onStatus func(name string, err error) // after every connect attempt

	mu    sync.Mutex
	pools map[string]*pool

	notices sync.Map // *pgconn.PgConn → *[]string, while a run collects notices
}

type pool struct {
	cfg config.Connection
	p   *pgxpool.Pool

	typesMu sync.Mutex
	types   map[uint32]string
}

func NewManager(password func(name string) (string, error), onStatus func(name string, err error)) *Manager {
	return &Manager{password: password, onStatus: onStatus, pools: map[string]*pool{}}
}

// Invalidate drops the pool of name, so the next use rebuilds it (after the
// Connection or its password changed, or it was deleted).
func (m *Manager) Invalidate(name string) {
	m.mu.Lock()
	p := m.pools[name]
	delete(m.pools, name)
	m.mu.Unlock()
	if p != nil {
		go p.p.Close() // waits for acquired conns
	}
}

func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, p := range m.pools {
		p.p.Close()
		delete(m.pools, name)
	}
}

func quoteDSN(v string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(v) + "'"
}

func (m *Manager) pool(name string, c config.Connection) (*pool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p := m.pools[name]; p != nil {
		if p.cfg == c {
			return p, nil
		}
		go p.p.Close()
		delete(m.pools, name)
	}
	password, err := m.password(name)
	if err != nil {
		return nil, err
	}
	kv := []string{
		"host=" + quoteDSN(c.Host),
		"port=" + strconv.Itoa(c.Port),
		"dbname=" + quoteDSN(c.Database),
		"user=" + quoteDSN(c.Role),
		"sslmode=" + quoteDSN(c.SSLMode),
		"application_name=yaam",
	}
	if password != "" {
		kv = append(kv, "password="+quoteDSN(password))
	}
	pc, err := pgxpool.ParseConfig(strings.Join(kv, " "))
	if err != nil {
		return nil, err
	}
	// release ends every run with DISCARD ALL, which drops the session's
	// prepared statements, so yaam's own queries must not use pgx's cache.
	pc.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeDescribeExec
	pc.ConnConfig.OnNotice = func(conn *pgconn.PgConn, n *pgconn.Notice) {
		if sink, ok := m.notices.Load(conn); ok {
			s := sink.(*[]string)
			*s = append(*s, n.Severity+": "+n.Message)
		}
	}
	p, err := pgxpool.NewWithConfig(context.Background(), pc)
	if err != nil {
		return nil, err
	}
	m.pools[name] = &pool{cfg: c, p: p}
	return m.pools[name], nil
}

// acquire gets a conn of Connection name and reports the connect outcome.
func (m *Manager) acquire(ctx context.Context, name string, c config.Connection) (*pool, *pgxpool.Conn, error) {
	p, err := m.pool(name, c)
	if err == nil {
		var conn *pgxpool.Conn
		if conn, err = p.p.Acquire(ctx); err == nil {
			m.onStatus(name, nil)
			return p, conn, nil
		}
	}
	if ctx.Err() == nil {
		m.onStatus(name, err)
	}
	return nil, nil, err
}

// Test connects and runs SELECT 1.
func (m *Manager) Test(ctx context.Context, name string, c config.Connection) error {
	_, conn, err := m.acquire(ctx, name, c)
	if err != nil {
		return err
	}
	defer conn.Release()
	_, err = conn.Exec(ctx, "SELECT 1")
	return err
}

func cleanupCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
}

// simple runs yaam's own statement on pc.
func simple(ctx context.Context, pc *pgconn.PgConn, sql string) error {
	_, err := pc.Exec(ctx, sql).ReadAll()
	return err
}

// release runs DISCARD ALL so no setting or temp table outlives the run,
// closing the conn when that fails, then returns it to the pool.
func release(ctx context.Context, conn *pgxpool.Conn) {
	ctx, cancel := cleanupCtx(ctx)
	defer cancel()
	pc := conn.Conn().PgConn()
	if pc.IsClosed() || pc.TxStatus() != 'I' || simple(ctx, pc, "DISCARD ALL") != nil {
		conn.Conn().Close(ctx)
	}
	conn.Release()
}

// execAll runs sql as one simple-protocol (text format) multi-statement
// query, collecting every completed statement.
func execAll(ctx context.Context, pc *pgconn.PgConn, sql string) ([]Statement, error) {
	stmts := []Statement{}
	mrr := pc.Exec(ctx, sql)
	var stmtErr error
	for mrr.NextResult() {
		rr := mrr.ResultReader()
		st := Statement{Columns: []Column{}, Rows: [][]*string{}}
		for _, fd := range rr.FieldDescriptions() {
			st.Columns = append(st.Columns, Column{Name: fd.Name, typeOID: fd.DataTypeOID, tableOID: fd.TableOID, attnum: fd.TableAttributeNumber})
		}
		for rr.NextRow() {
			vals := rr.Values()
			row := make([]*string, len(vals))
			for i, v := range vals {
				if v != nil {
					s := string(v)
					row[i] = &s
				}
			}
			st.Rows = append(st.Rows, row)
		}
		tag, err := rr.Close()
		if err != nil {
			stmtErr = err
			break
		}
		st.Command = tag.String()
		st.RowCount = tag.RowsAffected()
		if len(st.Columns) > 0 {
			st.RowCount = int64(len(st.Rows)) // SHOW, EXPLAIN… carry no count in their tag
		}
		stmts = append(stmts, st)
	}
	if err := mrr.Close(); err != nil {
		return stmts, err
	}
	return stmts, stmtErr
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

// collectNotices routes pc's notices into the returned slice until stop.
func (m *Manager) collectNotices(pc *pgconn.PgConn) (*[]string, func()) {
	sink := &[]string{}
	m.notices.Store(pc, sink)
	return sink, func() { m.notices.Delete(pc) }
}

// RunAgent runs an Agent Request (ADR 0005): one transaction with the
// Connection's statement_timeout, committed when every statement succeeds.
// ran is false when the SQL never reached PostgreSQL.
func (m *Manager) RunAgent(ctx context.Context, name string, c config.Connection, sql string) (res Result, ran bool) {
	res = newResult()
	p, conn, err := m.acquire(ctx, name, c)
	if err != nil {
		res.Error = err.Error()
		return res, false
	}
	defer release(ctx, conn)
	pc := conn.Conn().PgConn()
	notices, stop := m.collectNotices(pc)
	defer stop()

	if err := simple(ctx, pc, fmt.Sprintf("BEGIN; SET LOCAL statement_timeout = '%ds'", c.AgentTimeoutSeconds)); err != nil {
		res.Error = err.Error()
		return res, false
	}
	start := time.Now()
	res.Statements, err = execAll(ctx, pc, sql)
	if err == nil {
		err = simple(ctx, pc, "COMMIT")
	} else {
		cctx, cancel := cleanupCtx(ctx)
		simple(cctx, pc, "ROLLBACK")
		cancel()
	}
	res.DurationMs = ms(time.Since(start))
	if err != nil {
		res.Error = errorText(err)
	}
	stop()
	res.Notices = append(res.Notices, *notices...)
	p.nameTypes(ctx, conn, res.Statements)
	return res, true
}

// RunUser runs the User's editor SQL like psql would: no yaam transaction,
// so the User's own BEGIN/COMMIT work; a transaction left open is rolled
// back. It also reports whether the result can be edited in the grid.
func (m *Manager) RunUser(ctx context.Context, name string, c config.Connection, sql string) (res Result, ed *Editable, ran bool) {
	res = newResult()
	p, conn, err := m.acquire(ctx, name, c)
	if err != nil {
		res.Error = err.Error()
		return res, nil, false
	}
	defer release(ctx, conn)
	pc := conn.Conn().PgConn()
	notices, stop := m.collectNotices(pc)
	defer stop()

	start := time.Now()
	res.Statements, err = execAll(ctx, pc, sql)
	res.DurationMs = ms(time.Since(start))
	if err != nil {
		res.Error = errorText(err)
	}
	stop()
	res.Notices = append(res.Notices, *notices...)
	if !pc.IsClosed() && pc.TxStatus() != 'I' {
		cctx, cancel := cleanupCtx(ctx)
		simple(cctx, pc, "ROLLBACK")
		cancel()
		res.Notices = append(res.Notices, "an open transaction was rolled back")
	}
	if pc.IsClosed() {
		return res, nil, true
	}
	p.nameTypes(ctx, conn, res.Statements)
	if res.Error == "" && len(res.Statements) == 1 && len(res.Statements[0].Columns) > 0 {
		ed = detectEditable(ctx, conn, res.Statements[0].Columns)
	}
	return res, ed, true
}

// errorText is the PostgreSQL message with its SQLSTATE.
func errorText(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Error()
	}
	return err.Error()
}

// nameTypes fills in column type names from the Connection's cached
// pg_type, reloading it once when an OID is unknown.
func (p *pool) nameTypes(ctx context.Context, conn *pgxpool.Conn, stmts []Statement) {
	p.typesMu.Lock()
	defer p.typesMu.Unlock()
	reloaded := false
	for i := range stmts {
		for j := range stmts[i].Columns {
			col := &stmts[i].Columns[j]
			name, ok := p.types[col.typeOID]
			if !ok && !reloaded && !conn.Conn().PgConn().IsClosed() {
				reloaded = true
				p.loadTypes(ctx, conn)
				name, ok = p.types[col.typeOID]
			}
			if !ok {
				name = strconv.FormatUint(uint64(col.typeOID), 10)
			}
			col.Type = name
		}
	}
}

func (p *pool) loadTypes(ctx context.Context, conn *pgxpool.Conn) {
	rows, err := conn.Query(ctx, "SELECT oid, format_type(oid, NULL) FROM pg_type")
	if err != nil {
		return
	}
	defer rows.Close()
	types := map[uint32]string{}
	for rows.Next() {
		var oid uint32
		var name string
		if rows.Scan(&oid, &name) != nil {
			return
		}
		types[oid] = name
	}
	if rows.Err() == nil {
		p.types = types
	}
}
