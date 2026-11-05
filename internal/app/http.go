package app

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/wk-j/yaam/internal/approval"
	"github.com/wk-j/yaam/internal/config"
	"github.com/wk-j/yaam/internal/db"
	"github.com/wk-j/yaam/internal/keychain"
	"github.com/wk-j/yaam/internal/logstore"
	"github.com/wk-j/yaam/internal/webui"
)

const (
	maxBody    = 1 << 20
	maxRunBody = 8 << 20
)

var connectionName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,62}$`)

var sslModes = map[string]bool{"disable": true, "require": true, "verify-ca": true, "verify-full": true}

// Handler is the web UI and its API. hosts are the Host header values the
// listener answers to; token is the per-run bearer token (ADR 0002).
func (a *App) Handler(token string, hosts []string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /", webui.Handler())
	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, a.state()) })
	mux.HandleFunc("GET /api/events", a.events)
	mux.HandleFunc("PUT /api/connections/{name}", a.putConnection)
	mux.HandleFunc("DELETE /api/connections/{name}", a.deleteConnection)
	mux.HandleFunc("POST /api/connections/{name}/test", a.testConnection)
	mux.HandleFunc("POST /api/connections/{name}/rename", a.renameConnection)
	mux.HandleFunc("GET /api/connections/{name}/schema", a.schema)
	mux.HandleFunc("POST /api/connections/{name}/run", a.run)
	mux.HandleFunc("POST /api/connections/{name}/edits", a.edits)
	mux.HandleFunc("GET /api/agent-log", a.agentLog)
	mux.HandleFunc("GET /api/execution-log", a.executionLog)
	mux.HandleFunc("POST /api/pending/{id}/approve", a.decide(true))
	mux.HandleFunc("POST /api/pending/{id}/reject", a.decide(false))
	mux.HandleFunc("PUT /api/settings", a.putSettings)
	return guard(mux, []byte(token), hosts)
}

// guard rejects requests that did not come from this server's own page,
// as tantalum's web UI does:
//   - Host must be this server, which defeats DNS rebinding;
//   - Origin, when present, must be this server, which blocks other sites;
//   - /api/ requires the bearer token, which browsers only attach
//     cross-origin after a CORS preflight this server never approves.
func guard(next http.Handler, token []byte, hostList []string) http.Handler {
	hosts := map[string]bool{}
	for _, h := range hostList {
		hosts[h] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hosts[r.Host] {
			writeError(w, http.StatusForbidden, "unexpected Host header")
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			host, ok := strings.CutPrefix(origin, "http://")
			if !ok || !hosts[host] {
				writeError(w, http.StatusForbidden, "cross-origin request")
				return
			}
		}
		h := w.Header()
		// CodeMirror injects a <style> element at runtime, hence style-src.
		h.Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			h.Set("Cache-Control", "no-store")
			got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || subtle.ConstantTimeCompare([]byte(got), token) != 1 {
				writeError(w, http.StatusUnauthorized, "missing or wrong token; open yaam with `yaam open`")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func readJSON(w http.ResponseWriter, r *http.Request, limit int64, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit)).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return false
	}
	return true
}

// lookup finds the Connection of the {name} path value or answers 404.
func (a *App) lookup(w http.ResponseWriter, r *http.Request) (string, config.Connection, bool) {
	name := r.PathValue("name")
	c, ok := a.connection(name)
	if !ok {
		writeError(w, http.StatusNotFound, errUnknownConnection(name).Error())
	}
	return name, c, ok
}

func (a *App) putConnection(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !connectionName.MatchString(name) {
		writeError(w, http.StatusBadRequest, "invalid Connection name: use a-z, 0-9, _ and -, starting with a letter or digit, up to 63 characters")
		return
	}
	var body struct {
		Host                string  `json:"host"`
		Port                int     `json:"port"`
		Database            string  `json:"database"`
		Role                string  `json:"role"`
		SSLMode             string  `json:"sslmode"`
		AgentTimeoutSeconds int     `json:"agentTimeoutSeconds"`
		Password            *string `json:"password"`
	}
	if !readJSON(w, r, maxBody, &body) {
		return
	}
	c := config.Connection{
		Host: strings.TrimSpace(body.Host), Port: body.Port, Database: strings.TrimSpace(body.Database),
		Role: strings.TrimSpace(body.Role), SSLMode: body.SSLMode, AgentTimeoutSeconds: body.AgentTimeoutSeconds,
	}
	if c.Port == 0 {
		c.Port = config.DefaultPort
	}
	if c.SSLMode == "" {
		c.SSLMode = "disable"
	}
	if c.AgentTimeoutSeconds == 0 {
		c.AgentTimeoutSeconds = config.DefaultAgentTimeout
	}
	switch {
	case c.Host == "" || c.Database == "" || c.Role == "":
		writeError(w, http.StatusBadRequest, "host, database and role are required")
		return
	case c.Port < 1 || c.Port > 65535:
		writeError(w, http.StatusBadRequest, "port must be 1-65535")
		return
	case !sslModes[c.SSLMode]:
		writeError(w, http.StatusBadRequest, "sslmode must be disable, require, verify-ca or verify-full")
		return
	case c.AgentTimeoutSeconds < 1 || c.AgentTimeoutSeconds > 86400:
		writeError(w, http.StatusBadRequest, "agentTimeoutSeconds must be 1-86400")
		return
	case body.Password != nil && !keychain.ValidPassword(*body.Password):
		writeError(w, http.StatusBadRequest, "password must be printable ASCII")
		return
	}

	a.mu.Lock()
	err := a.storeConnection(name, c, body.Password)
	a.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.db.Invalidate(name)
	a.publishState()
	w.WriteHeader(http.StatusNoContent)
}

// storeConnection saves the password change (nil = keep) and the
// Connection. Callers hold a.mu.
func (a *App) storeConnection(name string, c config.Connection, password *string) error {
	if password != nil {
		if *password == "" {
			if err := a.keys.Delete(name); err != nil {
				return err
			}
		} else if err := a.keys.Set(name, *password); err != nil {
			return err
		}
		a.password[name] = *password != ""
	}
	next := a.cloneConfig()
	next.Connections[name] = c
	if err := a.saveConfig(next); err != nil {
		return err
	}
	delete(a.status, name)
	return nil
}

func (a *App) deleteConnection(w http.ResponseWriter, r *http.Request) {
	name, _, ok := a.lookup(w, r)
	if !ok {
		return
	}
	a.mu.Lock()
	err := a.keys.Delete(name)
	if err == nil {
		delete(a.password, name)
		next := a.cloneConfig()
		delete(next.Connections, name)
		err = a.saveConfig(next)
		delete(a.status, name)
	}
	a.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.db.Invalidate(name)
	a.publishState()
	w.WriteHeader(http.StatusNoContent)
}

var errNameTaken = errors.New("a Connection with that name already exists")

// renameConnection moves a Connection and its Keychain password to a new
// name in one step; it never overwrites an existing Connection.
func (a *App) renameConnection(w http.ResponseWriter, r *http.Request) {
	old, _, ok := a.lookup(w, r)
	if !ok {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if !readJSON(w, r, maxBody, &body) {
		return
	}
	if !connectionName.MatchString(body.Name) {
		writeError(w, http.StatusBadRequest, "invalid Connection name: use a-z, 0-9, _ and -, starting with a letter or digit, up to 63 characters")
		return
	}
	if body.Name == old {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	a.mu.Lock()
	err := a.moveConnection(old, body.Name)
	a.mu.Unlock()
	switch {
	case errors.Is(err, errNameTaken):
		writeError(w, http.StatusConflict, err.Error())
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.db.Invalidate(old)
	a.publishState()
	w.WriteHeader(http.StatusNoContent)
}

// moveConnection copies the password to name, saves the config with the
// Connection under name, then removes the old password; a failure before
// the config is saved leaves everything under old. Callers hold a.mu.
func (a *App) moveConnection(old, name string) error {
	c, ok := a.cfg.Connections[old]
	if !ok {
		return errUnknownConnection(old)
	}
	if _, taken := a.cfg.Connections[name]; taken {
		return errNameTaken
	}
	password, err := a.keys.Get(old)
	hasPassword := err == nil
	if err != nil && !errors.Is(err, keychain.ErrNotFound) {
		return err
	}
	if hasPassword {
		if err := a.keys.Set(name, password); err != nil {
			return err
		}
	}
	next := a.cloneConfig()
	next.Connections[name] = c
	delete(next.Connections, old)
	if err := a.saveConfig(next); err != nil {
		if hasPassword {
			a.keys.Delete(name)
		}
		return err
	}
	a.password[name] = hasPassword
	delete(a.password, old)
	if s, ok := a.status[old]; ok {
		a.status[name] = s
		delete(a.status, old)
	}
	if hasPassword {
		if err := a.keys.Delete(old); err != nil {
			return fmt.Errorf("renamed, but the password stored under %q could not be removed from the Keychain: %w", old, err)
		}
	}
	return nil
}

func (a *App) testConnection(w http.ResponseWriter, r *http.Request) {
	name, c, ok := a.lookup(w, r)
	if !ok {
		return
	}
	res := struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}{OK: true}
	if err := a.db.Test(r.Context(), name, c); err != nil {
		res.OK, res.Error = false, err.Error()
	}
	writeJSON(w, res)
}

func (a *App) schema(w http.ResponseWriter, r *http.Request) {
	name, c, ok := a.lookup(w, r)
	if !ok {
		return
	}
	tables, err := a.db.Schema(r.Context(), name, c)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, map[string]any{"tables": tables})
}

func (a *App) run(w http.ResponseWriter, r *http.Request) {
	name, c, ok := a.lookup(w, r)
	if !ok {
		return
	}
	var body struct {
		SQL string `json:"sql"`
	}
	if !readJSON(w, r, maxRunBody, &body) {
		return
	}
	if strings.TrimSpace(body.SQL) == "" {
		writeError(w, http.StatusBadRequest, "no SQL to run")
		return
	}
	res, ed, ran := a.db.RunUser(r.Context(), name, c, body.SQL)
	if ran {
		var rows *int64
		if res.Error == "" {
			rows = ptr(res.TotalRows())
		}
		a.insertExecLog(&logstore.ExecEntry{
			Time: now(), By: "user", Connection: name, Source: "editor",
			SQL: body.SQL, DurationMs: res.DurationMs, Rows: rows, Error: res.Error,
		})
	}
	writeJSON(w, struct {
		db.Result
		Editable *db.Editable `json:"editable"`
	}{res, ed})
}

func (a *App) edits(w http.ResponseWriter, r *http.Request) {
	name, c, ok := a.lookup(w, r)
	if !ok {
		return
	}
	var body struct {
		Schema  string      `json:"schema"`
		Table   string      `json:"table"`
		Changes []db.Change `json:"changes"`
		Apply   bool        `json:"apply"`
	}
	if !readJSON(w, r, maxBody, &body) {
		return
	}
	stmts, err := db.BuildEdits(body.Schema, body.Table, body.Changes)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !body.Apply {
		writeJSON(w, map[string]any{"sql": stmts})
		return
	}
	runs, durationMs, err := a.db.ApplyEdits(r.Context(), name, c, stmts)
	res := struct {
		SQL        []string `json:"sql"`
		DurationMs float64  `json:"durationMs"`
		Error      string   `json:"error"`
	}{SQL: stmts, DurationMs: durationMs}
	if err != nil {
		res.Error = err.Error()
	}
	for _, run := range runs {
		a.insertExecLog(&logstore.ExecEntry{
			Time: now(), By: "user", Connection: name, Source: "grid",
			SQL: run.SQL, DurationMs: run.DurationMs, Rows: ptr(run.Rows),
		})
	}
	writeJSON(w, res)
}

// page reads ?before= and ?limit= (default 100, at most 500).
func page(r *http.Request) (int64, int) {
	before, _ := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit <= 0 {
		limit = 100
	}
	return before, min(limit, 500)
}

func (a *App) agentLog(w http.ResponseWriter, r *http.Request) {
	entries, err := a.logs.ListAgent(page(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"entries": entries})
}

func (a *App) executionLog(w http.ResponseWriter, r *http.Request) {
	entries, err := a.logs.ListExec(page(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"entries": entries})
}

func (a *App) decide(approve bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := a.broker.Decide(r.PathValue("id"), approve)
		switch {
		case errors.Is(err, approval.ErrNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case err != nil:
			writeError(w, http.StatusInternalServerError, err.Error())
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

func (a *App) putSettings(w http.ResponseWriter, r *http.Request) {
	var body settingsView
	if !readJSON(w, r, maxBody, &body) {
		return
	}
	valid := func(n int) bool { return n >= 1000 && n <= 10000000 }
	if !valid(body.AgentLogLimit) || !valid(body.ExecutionLogLimit) {
		writeError(w, http.StatusBadRequest, "log limits must be 1000-10000000")
		return
	}
	a.mu.Lock()
	next := a.cloneConfig()
	next.Settings = config.Settings{AgentLogLimit: body.AgentLogLimit, ExecutionLogLimit: body.ExecutionLogLimit}
	err := a.saveConfig(next)
	a.mu.Unlock()
	if err == nil {
		err = a.logs.SetLimits(body.AgentLogLimit, body.ExecutionLogLimit)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.publishState()
	w.WriteHeader(http.StatusNoContent)
}
