package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wk-j/yaam/internal/approval"
	"github.com/wk-j/yaam/internal/db"
	"github.com/wk-j/yaam/internal/keychain"
	"github.com/wk-j/yaam/internal/logstore"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	a, err := New(t.TempDir(), &keychain.Fake{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	return a
}

var testSession = &agentSession{id: "s_test", project: "/tmp/project", connection: "shop-dev", name: "test-agent"}

// fakeRun counts runs and returns one SELECT-like statement.
type fakeRun struct{ calls atomic.Int32 }

func (f *fakeRun) run(context.Context) (db.Result, bool) {
	f.calls.Add(1)
	return db.Result{Statements: []db.Statement{{Command: "DELETE 2", RowCount: 2}}, Notices: []string{}}, true
}

// startRequest sends a Write and waits until it is pending.
func startRequest(t *testing.T, a *App, ctx context.Context, f *fakeRun) (approval.Pending, <-chan toolResult) {
	t.Helper()
	done := make(chan toolResult, 1)
	go func() {
		done <- a.request(ctx, testSession, "test-agent", "DELETE FROM orders WHERE id < 3", "clean up test rows", nil, f.run)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if p := a.broker.List(); len(p) == 1 {
			return p[0], done
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the Write never became pending")
	return approval.Pending{}, nil
}

func lastAgentEntry(t *testing.T, a *App) logstore.AgentEntry {
	t.Helper()
	entries, err := a.logs.ListAgent(0, 10)
	if err != nil || len(entries) != 1 {
		t.Fatalf("agent log = %+v, %v; want one entry", entries, err)
	}
	return entries[0]
}

func execCount(t *testing.T, a *App) int {
	t.Helper()
	entries, err := a.logs.ListExec(0, 10)
	if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}

func TestApproveRuns(t *testing.T) {
	a := newTestApp(t)
	f := &fakeRun{}
	p, done := startRequest(t, a, context.Background(), f)
	if e := lastAgentEntry(t, a); e.Status != "waiting" {
		t.Fatalf("status while pending = %q, want waiting", e.Status)
	}
	if f.calls.Load() != 0 {
		t.Fatal("the Write ran before Approval")
	}
	if err := a.broker.Decide(p.ID, true); err != nil {
		t.Fatal(err)
	}
	res := <-done
	if res.isError || f.calls.Load() != 1 {
		t.Fatalf("approved result %+v after %d runs, want one successful run", res, f.calls.Load())
	}
	if e := lastAgentEntry(t, a); e.Status != "run" || e.Rows == nil || *e.Rows != 2 {
		t.Fatalf("agent log after approval = %+v", e)
	}
	if n := execCount(t, a); n != 1 {
		t.Fatalf("execution log has %d entries, want 1", n)
	}
}

func TestRejectNeverRuns(t *testing.T) {
	a := newTestApp(t)
	f := &fakeRun{}
	p, done := startRequest(t, a, context.Background(), f)
	if err := a.broker.Decide(p.ID, false); err != nil {
		t.Fatal(err)
	}
	res := <-done
	if !res.isError || res.text != "rejected by the User" || f.calls.Load() != 0 {
		t.Fatalf("rejected result %+v after %d runs", res, f.calls.Load())
	}
	if e := lastAgentEntry(t, a); e.Status != "rejected" {
		t.Fatalf("agent log status %q, want rejected", e.Status)
	}
	if n := execCount(t, a); n != 0 {
		t.Fatalf("execution log has %d entries, want 0", n)
	}
}

func TestCancelledNeverRunsAndGoneIsNotFound(t *testing.T) {
	a := newTestApp(t)
	f := &fakeRun{}
	ctx, cancel := context.WithCancel(context.Background())
	p, done := startRequest(t, a, ctx, f)
	cancel()
	<-done
	if f.calls.Load() != 0 {
		t.Fatal("a cancelled Write ran")
	}
	if e := lastAgentEntry(t, a); e.Status != "cancelled" {
		t.Fatalf("agent log status %q, want cancelled", e.Status)
	}
	if n := len(a.broker.List()); n != 0 {
		t.Fatalf("%d items still pending after cancel", n)
	}
	if n := execCount(t, a); n != 0 {
		t.Fatalf("execution log has %d entries, want 0", n)
	}

	if err := a.broker.Decide(p.ID, true); !errors.Is(err, approval.ErrNotFound) {
		t.Fatalf("Decide on a gone item = %v, want ErrNotFound", err)
	}
	h := a.Handler("tok", []string{"127.0.0.1:7341"})
	req := httptest.NewRequest("POST", "http://127.0.0.1:7341/api/pending/"+p.ID+"/approve", nil)
	req.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("approve a gone item: HTTP %d, want 404", rec.Code)
	}
	if f.calls.Load() != 0 {
		t.Fatal("approving a gone item ran it")
	}
}

func TestRefusedAndReadPaths(t *testing.T) {
	a := newTestApp(t)
	f := &fakeRun{}
	res := a.request(context.Background(), testSession, "test-agent", "BEGIN; DELETE FROM orders; COMMIT", "tx", nil, f.run)
	if !res.isError || res.text != refusedTx || f.calls.Load() != 0 {
		t.Fatalf("txcontrol result %+v", res)
	}
	res = a.request(context.Background(), testSession, "test-agent", "SELECT 1", "", nil, f.run)
	if !res.isError || f.calls.Load() != 0 {
		t.Fatalf("empty purpose result %+v", res)
	}
	res = a.request(context.Background(), testSession, "test-agent", "SELECT * FROM orders", "look", nil, f.run)
	if res.isError || f.calls.Load() != 1 || len(a.broker.List()) != 0 {
		t.Fatalf("read result %+v, %d runs", res, f.calls.Load())
	}
}

// A Request still waiting when yaam stops can never be approved: its call is
// gone with the old process. The next start records it as cancelled (ADR 0003).
func TestRestartCancelsWaitingRequests(t *testing.T) {
	home := t.TempDir()
	a, err := New(home, &keychain.Fake{})
	if err != nil {
		t.Fatal(err)
	}
	waiting := &logstore.AgentEntry{Time: now(), Kind: "request", Status: "waiting"}
	finished := &logstore.AgentEntry{Time: now(), Kind: "request", Status: "run"}
	for _, e := range []*logstore.AgentEntry{waiting, finished} {
		if err := a.logs.InsertAgent(e); err != nil {
			t.Fatal(err)
		}
	}
	a.Close()

	a, err = New(home, &keychain.Fake{})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	entries, err := a.logs.ListAgent(0, 10)
	if err != nil {
		t.Fatal(err)
	}
	got := map[int64]string{}
	for _, e := range entries {
		got[e.ID] = e.Status
	}
	if got[waiting.ID] != "cancelled" || got[finished.ID] != "run" {
		t.Fatalf("statuses after restart = %v, want waiting→cancelled and run unchanged", got)
	}
}

func TestGuard(t *testing.T) {
	a := newTestApp(t)
	h := a.Handler("tok", []string{"127.0.0.1:7341"})
	cases := []struct {
		host, origin, auth string
		want               int
	}{
		{"127.0.0.1:7341", "", "Bearer tok", http.StatusOK},
		{"evil.example:7341", "", "Bearer tok", http.StatusForbidden},
		{"127.0.0.1:7341", "http://evil.example", "Bearer tok", http.StatusForbidden},
		{"127.0.0.1:7341", "", "Bearer nope", http.StatusUnauthorized},
		{"127.0.0.1:7341", "", "", http.StatusUnauthorized},
	}
	for _, c := range cases {
		req := httptest.NewRequest("GET", "http://"+c.host+"/api/state", nil)
		if c.origin != "" {
			req.Header.Set("Origin", c.origin)
		}
		if c.auth != "" {
			req.Header.Set("Authorization", c.auth)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != c.want {
			t.Errorf("host %s origin %q auth %q: HTTP %d, want %d", c.host, c.origin, c.auth, rec.Code, c.want)
		}
	}
}
