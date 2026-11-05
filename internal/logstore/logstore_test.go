package logstore

import (
	"path/filepath"
	"testing"
)

func open(t *testing.T, agentLimit, execLimit int) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "yaam.db"), agentLimit, execLimit)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestPruneKeepsNewestN(t *testing.T) {
	s := open(t, 5, 3)
	var lastAgent, lastExec int64
	for i := range 12 {
		a := &AgentEntry{Time: "t", Kind: "request", SQL: "SELECT 1", Status: "run"}
		if err := s.InsertAgent(a); err != nil {
			t.Fatal(err)
		}
		lastAgent = a.ID
		e := &ExecEntry{Time: "t", By: "user", Source: "editor", SQL: "SELECT 1", DurationMs: float64(i)}
		if err := s.InsertExec(e); err != nil {
			t.Fatal(err)
		}
		lastExec = e.ID
	}
	agents, err := s.ListAgent(0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 5 {
		t.Fatalf("agent log kept %d entries, want 5", len(agents))
	}
	for i, e := range agents {
		if want := lastAgent - int64(i); e.ID != want {
			t.Fatalf("agent entry %d has id %d, want %d", i, e.ID, want)
		}
	}
	execs, err := s.ListExec(0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(execs) != 3 || execs[0].ID != lastExec || execs[2].ID != lastExec-2 {
		t.Fatalf("execution log kept %+v, want the newest 3 ending at id %d", execs, lastExec)
	}

	// Lowering the limit prunes at once.
	if err := s.SetLimits(2, 1); err != nil {
		t.Fatal(err)
	}
	agents, _ = s.ListAgent(0, 100)
	execs, _ = s.ListExec(0, 100)
	if len(agents) != 2 || agents[0].ID != lastAgent || len(execs) != 1 || execs[0].ID != lastExec {
		t.Fatalf("after SetLimits(2, 1): agents %v, execs %v", agents, execs)
	}
}

func TestUpdateAndPage(t *testing.T) {
	s := open(t, 100, 100)
	for range 4 {
		if err := s.InsertAgent(&AgentEntry{Time: "t", Kind: "request", Status: "waiting"}); err != nil {
			t.Fatal(err)
		}
	}
	d, rows := 1.5, int64(3)
	if err := s.UpdateAgent(&AgentEntry{ID: 2, Status: "run", DurationMs: &d, Rows: &rows}); err != nil {
		t.Fatal(err)
	}
	page, err := s.ListAgent(3, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 2 || page[0].ID != 2 || page[0].Status != "run" || *page[0].Rows != 3 || page[1].DurationMs != nil {
		t.Fatalf("page before 3: %+v", page)
	}
}
