package app

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wk-j/yaam/internal/keychain"
)

func renameTestApp(t *testing.T) (*App, keychain.Store, func(method, path, body string) int) {
	t.Helper()
	keys := &keychain.Fake{}
	a, err := New(t.TempDir(), keys)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	h := a.Handler("tok", []string{"127.0.0.1:7341"})
	do := func(method, path, body string) int {
		req := httptest.NewRequest(method, "http://127.0.0.1:7341"+path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer tok")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	return a, keys, do
}

func putConn(t *testing.T, do func(string, string, string) int, name, password string) {
	t.Helper()
	body := `{"host":"db.example","port":5433,"database":"shop","role":"agent","sslmode":"require","agentTimeoutSeconds":45,"password":"` + password + `"}`
	if code := do("PUT", "/api/connections/"+name, body); code != http.StatusNoContent {
		t.Fatalf("PUT %s: HTTP %d", name, code)
	}
}

func TestRenameMovesConfigAndPassword(t *testing.T) {
	a, keys, do := renameTestApp(t)
	putConn(t, do, "shop-dev", "s3cret")

	if code := do("POST", "/api/connections/shop-dev/rename", `{"name":"shop-main"}`); code != http.StatusNoContent {
		t.Fatalf("rename: HTTP %d, want 204", code)
	}
	if _, ok := a.connection("shop-dev"); ok {
		t.Error("the old name still exists")
	}
	c, ok := a.connection("shop-main")
	if !ok || c.Host != "db.example" || c.Port != 5433 || c.SSLMode != "require" || c.AgentTimeoutSeconds != 45 {
		t.Errorf("shop-main = %+v (found %v), want the settings of shop-dev", c, ok)
	}
	if pw, err := keys.Get("shop-main"); err != nil || pw != "s3cret" {
		t.Errorf("password under the new name = %q, %v; want s3cret", pw, err)
	}
	if _, err := keys.Get("shop-dev"); !errors.Is(err, keychain.ErrNotFound) {
		t.Errorf("password under the old name: err = %v, want ErrNotFound", err)
	}

	// The move is saved: another yaam reading the same home knows only the new name.
	b, err := New(filepath.Dir(a.cfgPath), keys)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if _, ok := b.connection("shop-main"); !ok {
		t.Error("shop-main is missing from the saved config")
	}
	if _, ok := b.connection("shop-dev"); ok {
		t.Error("shop-dev is still in the saved config")
	}
}

func TestRenameRefusesTakenName(t *testing.T) {
	a, keys, do := renameTestApp(t)
	putConn(t, do, "shop-dev", "pw-dev")
	putConn(t, do, "shop-main", "pw-main")

	if code := do("POST", "/api/connections/shop-dev/rename", `{"name":"shop-main"}`); code != http.StatusConflict {
		t.Fatalf("rename onto an existing name: HTTP %d, want 409", code)
	}
	for name, want := range map[string]string{"shop-dev": "pw-dev", "shop-main": "pw-main"} {
		if _, ok := a.connection(name); !ok {
			t.Errorf("%s was removed", name)
		}
		if pw, err := keys.Get(name); err != nil || pw != want {
			t.Errorf("password of %s = %q, %v; want %q", name, pw, err, want)
		}
	}
}

func TestRenameRejectsBadRequests(t *testing.T) {
	_, _, do := renameTestApp(t)
	putConn(t, do, "shop-dev", "")
	cases := map[string]struct {
		path, body string
		want       int
	}{
		"unknown Connection": {"/api/connections/nope/rename", `{"name":"other"}`, http.StatusNotFound},
		"invalid new name":   {"/api/connections/shop-dev/rename", `{"name":"Bad Name"}`, http.StatusBadRequest},
	}
	for name, c := range cases {
		if code := do("POST", c.path, c.body); code != c.want {
			t.Errorf("%s: HTTP %d, want %d", name, code, c.want)
		}
	}
}
