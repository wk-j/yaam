package db

import (
	"slices"
	"testing"
)

func str(s string) *string { return &s }

func TestBuildEditsQuoting(t *testing.T) {
	got, err := BuildEdits(`my"schema`, "orders", []Change{
		{Op: "update", Key: map[string]*string{"id": str("7")}, Set: map[string]*string{
			`odd"col`: str("O'Brien"),
			"note":    nil,
		}},
		{Op: "delete", Key: map[string]*string{"id": str("9"), "region": str("it's")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`UPDATE "my""schema"."orders" SET "note" = NULL, "odd""col" = 'O''Brien' WHERE "id" = '7';`,
		`DELETE FROM "my""schema"."orders" WHERE "id" = '9' AND "region" = 'it''s';`,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("BuildEdits =\n%q\nwant\n%q", got, want)
	}
}

func TestBuildEditsRejects(t *testing.T) {
	cases := map[string][]Change{
		"no key":       {{Op: "delete"}},
		"null key":     {{Op: "delete", Key: map[string]*string{"id": nil}}},
		"empty update": {{Op: "update", Key: map[string]*string{"id": str("1")}}},
		"unknown op":   {{Op: "insert", Key: map[string]*string{"id": str("1")}}},
		"no changes":   {},
	}
	for name, changes := range cases {
		if got, err := BuildEdits("public", "t", changes); err == nil {
			t.Errorf("%s: BuildEdits = %q, want an error", name, got)
		}
	}
}
