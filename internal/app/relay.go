package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/wk-j/yaam/internal/db"
)

// Version is reported as the MCP server version.
const Version = "1.0.0"

// Hello is the first line `yaam mcp` sends on the relay socket; the main
// process answers with one HelloReply line.
type Hello struct {
	Connection string `json:"connection"`
	Cwd        string `json:"cwd"`
}

type HelloReply struct {
	OK    bool   `json:"ok,omitempty"`
	Error string `json:"error,omitempty"`
}

// ServeRelay accepts relay connections on ln until it is closed.
func (a *App) ServeRelay(ln net.Listener) error {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		go a.relay(conn)
	}
}

// relayConn is the MCP transport side of a relay socket: reads continue
// from the buffered reader that consumed the hello line.
type relayConn struct {
	*bufio.Reader
	net.Conn
}

func (c relayConn) Read(p []byte) (int, error) { return c.Reader.Read(p) }

func (a *App) relay(conn net.Conn) {
	defer conn.Close()
	br := bufio.NewReader(conn)
	line, err := br.ReadSlice('\n')
	if err != nil {
		return
	}
	var hello Hello
	reply := func(r HelloReply) error {
		b, _ := json.Marshal(r)
		_, err := conn.Write(append(b, '\n'))
		return err
	}
	if err := json.Unmarshal(line, &hello); err != nil {
		reply(HelloReply{Error: "bad hello line from the relay"})
		return
	}
	c, ok := a.connection(hello.Connection)
	if !ok {
		reply(HelloReply{Error: errUnknownConnection(hello.Connection).Error()})
		return
	}
	if reply(HelloReply{OK: true}) != nil {
		return
	}

	s := &agentSession{id: sessionID(), project: hello.Cwd, connection: hello.Connection, since: now(), name: unknownAgent}
	a.addAgent(s)
	defer a.removeAgent(s)
	server := a.mcpServer(s, c.AgentTimeoutSeconds)
	rc := relayConn{br, conn}
	ss, err := server.Connect(context.Background(), &mcp.IOTransport{Reader: rc, Writer: rc}, nil)
	if err != nil {
		logError("relay", err)
		return
	}
	ss.Wait()
}

type runSQLInput struct {
	SQL     string `json:"sql" jsonschema:"the SQL of the Request; several statements separated by semicolons run as one transaction"`
	Purpose string `json:"purpose" jsonschema:"required one line saying why you send this Request; the User sees it when approving"`
}

type searchSchemaInput struct {
	Pattern string `json:"pattern" jsonschema:"case-insensitive substring of a table, view, column, function or index name"`
}

func textResult(r toolResult) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: r.text}}, IsError: r.isError}
}

// agentNameOf is the Agent's MCP client name, falling back to the session's.
func agentNameOf(req *mcp.CallToolRequest, s *agentSession) string {
	if info := req.ClientInfo(); info != nil && info.Name != "" {
		return info.Name
	}
	return s.agentName()
}

// mcpServer is the MCP server of one relay session, bound to its
// Connection (ADR 0007), with exactly two tools.
func (a *App) mcpServer(s *agentSession, timeoutSeconds int) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "yaam", Version: Version}, &mcp.ServerOptions{
		InitializedHandler: func(ctx context.Context, req *mcp.InitializedRequest) {
			if p := req.Session.InitializeParams(); p != nil && p.ClientInfo != nil {
				s.setAgentName(p.ClientInfo.Name)
				a.publishState()
			}
		},
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "run_sql",
		Description: fmt.Sprintf(strings.Join([]string{
			"Run SQL on the PostgreSQL Connection %q.",
			"Reads (SELECT without a locking clause or data-changing WITH, SHOW, EXPLAIN without ANALYZE) run immediately.",
			"Any Write (every other statement, including INSERT, UPDATE, DELETE, DDL, CALL, DO, COPY and SET) waits for the User's Approval in yaam's web UI; this call stays open until the User approves or rejects it.",
			"The whole SQL runs as one transaction, committed only when every statement succeeds, so never include BEGIN, COMMIT or ROLLBACK.",
			"yaam does not limit returned rows, so always add LIMIT to queries that may return many rows.",
			"Each Request may run for at most %d seconds.",
			"purpose is required: one line telling the User why you send this Request.",
			"The result is JSON: statements (command, columns with types, rows as PostgreSQL text with null for NULL, rowCount), durationMs, error, notices.",
		}, " "), s.connection, timeoutSeconds),
	}, func(ctx context.Context, req *mcp.CallToolRequest, in runSQLInput) (*mcp.CallToolResult, any, error) {
		var progress func()
		if tok := req.Params.GetProgressToken(); tok != nil {
			n := 0.0
			progress = func() {
				n++
				req.Session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{ProgressToken: tok, Progress: n, Message: waitingMsg})
			}
		}
		run := func(ctx context.Context) (db.Result, bool) {
			c, ok := a.connection(s.connection)
			if !ok {
				res := db.Result{Statements: []db.Statement{}, Notices: []string{}, Error: errUnknownConnection(s.connection).Error()}
				return res, false
			}
			return a.db.RunAgent(ctx, s.connection, c, in.SQL)
		}
		return textResult(a.request(ctx, s, agentNameOf(req, s), in.SQL, in.Purpose, progress, run)), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "search_schema",
		Description: fmt.Sprintf("Search the schema of the PostgreSQL Connection %q: a case-insensitive substring match over "+
			"tables and views (by name or column name), functions and indexes in non-system schemas. "+
			"Returns at most 50 lines such as `table public.orders (id bigint pk, customer text)`.", s.connection),
	}, func(ctx context.Context, req *mcp.CallToolRequest, in searchSchemaInput) (*mcp.CallToolResult, any, error) {
		return textResult(a.searchSchema(ctx, s, agentNameOf(req, s), in.Pattern)), nil, nil
	})
	return server
}
