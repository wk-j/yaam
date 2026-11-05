// Package app is yaam's main process: Connections, the Agent Log and the
// Execution Log, Approval, the HTTP API with its event stream, and the
// relay endpoint where each `yaam mcp` gets its own MCP server.
package app

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/wk-j/yaam/internal/approval"
	"github.com/wk-j/yaam/internal/config"
	"github.com/wk-j/yaam/internal/db"
	"github.com/wk-j/yaam/internal/keychain"
	"github.com/wk-j/yaam/internal/logstore"
)

// App holds the state of the main process.
type App struct {
	cfgPath string
	keys    keychain.Store
	logs    *logstore.Store
	db      *db.Manager
	broker  *approval.Broker
	hub     *hub

	// progressInterval is how often a Request waiting for Approval sends a
	// progress notification (when its call carries a progress token).
	progressInterval time.Duration

	mu       sync.Mutex
	cfg      *config.Config
	password map[string]bool // cached Keychain presence per Connection
	status   map[string]connStatus
	agents   map[string]*agentSession
}

type connStatus struct {
	status string // ok | error | unknown
	err    string
}

// New opens the files under home (which must exist).
func New(home string, keys keychain.Store) (*App, error) {
	cfgPath := filepath.Join(home, "config.toml")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, err
	}
	logs, err := logstore.Open(filepath.Join(home, "yaam.db"), cfg.Settings.AgentLogLimit, cfg.Settings.ExecutionLogLimit)
	if err != nil {
		return nil, err
	}
	// Requests left waiting by a previous run lost their tool call with that
	// process, so they can never run (ADR 0003).
	if err := logs.CancelWaiting(); err != nil {
		logs.Close()
		return nil, err
	}
	a := &App{
		cfgPath:          cfgPath,
		keys:             keys,
		logs:             logs,
		hub:              newHub(),
		progressInterval: 15 * time.Second,
		cfg:              cfg,
		password:         map[string]bool{},
		status:           map[string]connStatus{},
		agents:           map[string]*agentSession{},
	}
	a.broker = approval.NewBroker(a.publishState)
	a.db = db.NewManager(a.connPassword, a.setStatus)
	return a, nil
}

func (a *App) Close() {
	a.db.Close()
	a.logs.Close()
}

func (a *App) connPassword(name string) (string, error) {
	p, err := a.keys.Get(name)
	if errors.Is(err, keychain.ErrNotFound) {
		return "", nil
	}
	return p, err
}

// setStatus records the outcome of a connect attempt.
func (a *App) setStatus(name string, err error) {
	s := connStatus{status: "ok"}
	if err != nil {
		s = connStatus{status: "error", err: err.Error()}
	}
	a.mu.Lock()
	changed := a.status[name] != s
	a.status[name] = s
	a.mu.Unlock()
	if changed {
		a.publishState()
	}
}

func (a *App) connection(name string) (config.Connection, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	c, ok := a.cfg.Connections[name]
	return c, ok
}

func now() string { return time.Now().Format(time.RFC3339) }

type connectionView struct {
	Name                string `json:"name"`
	Host                string `json:"host"`
	Port                int    `json:"port"`
	Database            string `json:"database"`
	Role                string `json:"role"`
	SSLMode             string `json:"sslmode"`
	AgentTimeoutSeconds int    `json:"agentTimeoutSeconds"`
	HasPassword         bool   `json:"hasPassword"`
	Status              string `json:"status"`
	StatusError         string `json:"statusError"`
}

type settingsView struct {
	AgentLogLimit     int `json:"agentLogLimit"`
	ExecutionLogLimit int `json:"executionLogLimit"`
}

type agentView struct {
	ID         string `json:"id"`
	AgentName  string `json:"agentName"`
	Project    string `json:"project"`
	Connection string `json:"connection"`
	Since      string `json:"since"`
}

type stateView struct {
	Connections []connectionView   `json:"connections"`
	Settings    settingsView       `json:"settings"`
	Agents      []agentView        `json:"agents"`
	Pending     []approval.Pending `json:"pending"`
}

// state is the /api/state payload; it never holds a password.
func (a *App) state() stateView {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := stateView{
		Connections: []connectionView{},
		Settings:    settingsView{a.cfg.Settings.AgentLogLimit, a.cfg.Settings.ExecutionLogLimit},
		Agents:      []agentView{},
		Pending:     a.broker.List(),
	}
	for _, name := range slices.Sorted(maps.Keys(a.cfg.Connections)) {
		c := a.cfg.Connections[name]
		has, ok := a.password[name]
		if !ok {
			has, _ = a.keys.Has(name)
			a.password[name] = has
		}
		st := a.status[name]
		if st.status == "" {
			st.status = "unknown"
		}
		s.Connections = append(s.Connections, connectionView{
			Name: name, Host: c.Host, Port: c.Port, Database: c.Database, Role: c.Role, SSLMode: c.SSLMode,
			AgentTimeoutSeconds: c.AgentTimeoutSeconds, HasPassword: has, Status: st.status, StatusError: st.err,
		})
	}
	for _, ag := range a.agents {
		s.Agents = append(s.Agents, ag.view())
	}
	slices.SortFunc(s.Agents, func(x, y agentView) int {
		return cmp.Or(cmp.Compare(x.Since, y.Since), cmp.Compare(x.ID, y.ID))
	})
	return s
}

func (a *App) publishState() { a.hub.publish("state", a.state()) }

// saveConfig writes next and makes it current, or leaves both unchanged.
// Callers hold a.mu.
func (a *App) saveConfig(next *config.Config) error {
	if err := config.Save(a.cfgPath, next); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	a.cfg = next
	return nil
}

func (a *App) cloneConfig() *config.Config {
	next := *a.cfg
	next.Connections = maps.Clone(a.cfg.Connections)
	return &next
}

func (a *App) insertAgentLog(e *logstore.AgentEntry) {
	if err := a.logs.InsertAgent(e); err != nil {
		logError("agent log", err)
		return
	}
	a.hub.publish("agent-log", e)
}

func (a *App) updateAgentLog(e *logstore.AgentEntry) {
	if e.ID == 0 {
		a.insertAgentLog(e)
		return
	}
	if err := a.logs.UpdateAgent(e); err != nil {
		logError("agent log", err)
		return
	}
	a.hub.publish("agent-log", e)
}

func (a *App) insertExecLog(e *logstore.ExecEntry) {
	if err := a.logs.InsertExec(e); err != nil {
		logError("execution log", err)
		return
	}
	a.hub.publish("execution-log", e)
}
