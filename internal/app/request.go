package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/wk-j/yaam/internal/approval"
	"github.com/wk-j/yaam/internal/classify"
	"github.com/wk-j/yaam/internal/db"
	"github.com/wk-j/yaam/internal/logstore"
)

const (
	unknownAgent  = "unknown agent"
	refusedTx     = "A Request cannot start or end a transaction; send it again without BEGIN/COMMIT/ROLLBACK"
	rejected      = "rejected by the User"
	waitingMsg    = `Waiting for Approval in yaam (run "yaam open")`
	purposeNeeded = "purpose is required: one line saying why you send this Request"
)

func logError(what string, err error) { log.Printf("yaam: %s: %v", what, err) }

func sessionID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "s_" + hex.EncodeToString(b)
}

// agentSession is one connected relay: an Agent in a project bound to one
// Connection.
type agentSession struct {
	id         string
	project    string
	connection string
	since      string

	mu   sync.Mutex
	name string // MCP clientInfo.name, once initialized
}

func (s *agentSession) agentName() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.name
}

func (s *agentSession) setAgentName(name string) {
	if name == "" {
		return
	}
	s.mu.Lock()
	s.name = name
	s.mu.Unlock()
}

func (s *agentSession) view() agentView {
	return agentView{ID: s.id, AgentName: s.agentName(), Project: s.project, Connection: s.connection, Since: s.since}
}

func (a *App) addAgent(s *agentSession) {
	a.mu.Lock()
	a.agents[s.id] = s
	a.mu.Unlock()
	a.publishState()
}

func (a *App) removeAgent(s *agentSession) {
	a.mu.Lock()
	delete(a.agents, s.id)
	a.mu.Unlock()
	a.publishState()
}

// toolResult is what an MCP tool call answers.
type toolResult struct {
	text    string
	isError bool
}

func compactJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func ptr[T any](v T) *T { return &v }

// request handles one Agent Request end to end: classify, refuse or wait
// for Approval, run, and log. run executes the SQL; ran reports whether it
// reached PostgreSQL. progress (nil without a progress token) is called
// every progressInterval while the Request waits for Approval.
func (a *App) request(ctx context.Context, s *agentSession, agentName, sql, purpose string,
	progress func(), run func(context.Context) (db.Result, bool)) toolResult {
	e := &logstore.AgentEntry{
		Time: now(), AgentName: agentName, Project: s.project, Connection: s.connection,
		Kind: "request", Purpose: purpose, SQL: sql, Status: "run",
	}
	fail := func(msg string) toolResult {
		e.Status, e.Error = "failed", msg
		a.updateAgentLog(e)
		return toolResult{msg, true}
	}
	if strings.TrimSpace(purpose) == "" || strings.ContainsAny(purpose, "\r\n") {
		return fail(purposeNeeded)
	}
	stmts, err := classify.Parse(sql)
	if err != nil {
		return fail(err.Error())
	}
	if len(stmts) == 0 {
		return fail("the Request holds no SQL statement")
	}
	kinds := make([]classify.Kind, len(stmts))
	for i, st := range stmts {
		kinds[i] = st.Kind
	}
	if slices.Contains(kinds, classify.TxControl) {
		return fail(refusedTx)
	}

	if slices.Contains(kinds, classify.Write) {
		e.Status = "waiting"
		a.insertAgentLog(e)
		approved, err := a.waitForApproval(ctx, approval.Pending{
			ID: approval.NewID(), Time: e.Time, AgentName: agentName, Project: s.project,
			Connection: s.connection, Purpose: purpose, SQL: sql, Statements: stmts,
		}, progress)
		if err != nil || ctx.Err() != nil {
			e.Status = "cancelled"
			a.updateAgentLog(e)
			return toolResult{"cancelled before the User decided", true}
		}
		if !approved {
			e.Status = "rejected"
			a.updateAgentLog(e)
			return toolResult{rejected, true}
		}
		e.Status = "run"
		a.updateAgentLog(e)
	} else {
		a.insertAgentLog(e)
	}

	res, ran := run(ctx)
	var rows *int64
	if res.Error == "" {
		rows = ptr(res.TotalRows())
	}
	if ran {
		a.insertExecLog(&logstore.ExecEntry{
			Time: now(), By: "agent", AgentName: agentName, Project: s.project, Connection: s.connection,
			Source: "agent", SQL: sql, DurationMs: res.DurationMs, Rows: rows, Error: res.Error,
		})
		e.DurationMs = ptr(res.DurationMs)
	}
	e.Rows, e.Error = rows, res.Error
	switch {
	case res.Error == "":
		e.Status = "run"
	case !ran && ctx.Err() != nil:
		e.Status = "cancelled"
	default:
		e.Status = "failed"
	}
	a.updateAgentLog(e)
	return toolResult{compactJSON(res), res.Error != ""}
}

func (a *App) waitForApproval(ctx context.Context, p approval.Pending, progress func()) (bool, error) {
	if progress == nil {
		return a.broker.Wait(ctx, p)
	}
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		t := time.NewTicker(a.progressInterval)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				progress()
			}
		}
	}()
	return a.broker.Wait(ctx, p)
}

// searchSchema is the search_schema tool: logged in the Agent Log as kind
// schema, never in the Execution Log.
func (a *App) searchSchema(ctx context.Context, s *agentSession, agentName, pattern string) toolResult {
	e := &logstore.AgentEntry{
		Time: now(), AgentName: agentName, Project: s.project, Connection: s.connection,
		Kind: "schema", SQL: pattern, Status: "run",
	}
	defer a.insertAgentLog(e)
	c, ok := a.connection(s.connection)
	if !ok {
		e.Status, e.Error = "failed", errUnknownConnection(s.connection).Error()
		return toolResult{e.Error, true}
	}
	start := time.Now()
	lines, err := a.db.Search(ctx, s.connection, c, pattern)
	e.DurationMs = ptr(float64(time.Since(start).Microseconds()) / 1000)
	if err != nil {
		e.Status, e.Error = "failed", err.Error()
		return toolResult{e.Error, true}
	}
	e.Rows = ptr(int64(len(lines)))
	if len(lines) == 0 {
		return toolResult{"no matches", false}
	}
	return toolResult{strings.Join(lines, "\n"), false}
}

func errUnknownConnection(name string) error {
	return errors.New(`unknown Connection "` + name + `"; add it in yaam's web UI`)
}
