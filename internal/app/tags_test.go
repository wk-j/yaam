package app

import (
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func putConnTags(t *testing.T, do func(string, string, string) int, name, tagsJSON string) int {
	t.Helper()
	body := `{"host":"db.example","database":"shop","role":"agent","tags":` + tagsJSON + `}`
	return do("PUT", "/api/connections/"+name, body)
}

func stateTags(a *App, name string) ([]string, bool) {
	for _, c := range a.state().Connections {
		if c.Name == name {
			return c.Tags, true
		}
	}
	return nil, false
}

func TestTagsAreNormalizedSavedAndShown(t *testing.T) {
	a, keys, do := renameTestApp(t)
	if code := putConnTags(t, do, "shop-dev", `["  Prod ", "shop  team", "", "prod", "งานขาย"]`); code != http.StatusNoContent {
		t.Fatalf("PUT: HTTP %d, want 204", code)
	}
	want := []string{"prod", "shop team", "งานขาย"}
	if got, _ := stateTags(a, "shop-dev"); !slices.Equal(got, want) {
		t.Errorf("state tags = %q, want %q", got, want)
	}

	b, err := New(filepath.Dir(a.cfgPath), keys)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if c, _ := b.connection("shop-dev"); !slices.Equal(c.Tags, want) {
		t.Errorf("saved tags = %q, want %q", c.Tags, want)
	}
}

func TestTagsFollowRenameAndClear(t *testing.T) {
	a, _, do := renameTestApp(t)
	putConnTags(t, do, "shop-dev", `["prod"]`)
	if code := do("POST", "/api/connections/shop-dev/rename", `{"name":"shop-main"}`); code != http.StatusNoContent {
		t.Fatalf("rename: HTTP %d", code)
	}
	if got, _ := stateTags(a, "shop-main"); !slices.Equal(got, []string{"prod"}) {
		t.Errorf("tags after rename = %q, want [prod]", got)
	}

	if code := putConnTags(t, do, "shop-main", `[]`); code != http.StatusNoContent {
		t.Fatalf("PUT with no tags: HTTP %d", code)
	}
	got, ok := stateTags(a, "shop-main")
	if !ok || got == nil || len(got) != 0 {
		t.Errorf("tags after clearing = %#v, want an empty list (never null in JSON)", got)
	}
}

func TestTagsRejectBadInput(t *testing.T) {
	a, _, do := renameTestApp(t)
	putConnTags(t, do, "shop-dev", `["prod"]`)
	cases := map[string]string{
		"comma":        `["a,b"]`,
		"too long":     `["` + strings.Repeat("ก", maxTagLen+1) + `"]`,
		"control char": `["a\u0007b"]`,
		"too many":     `["1","2","3","4","5","6","7","8","9","10","11"]`,
	}
	for name, tags := range cases {
		if code := putConnTags(t, do, "shop-dev", tags); code != http.StatusBadRequest {
			t.Errorf("%s: HTTP %d, want 400", name, code)
		}
	}
	if got, _ := stateTags(a, "shop-dev"); !slices.Equal(got, []string{"prod"}) {
		t.Errorf("a refused PUT changed the tags to %q", got)
	}
	// Exactly the limits are fine.
	if code := putConnTags(t, do, "shop-dev", `["`+strings.Repeat("ก", maxTagLen)+`","2","3","4","5","6","7","8","9","10"]`); code != http.StatusNoContent {
		t.Errorf("10 Tags of %d characters: HTTP %d, want 204", maxTagLen, code)
	}
}
