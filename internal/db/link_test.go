package db

import (
	"testing"

	"github.com/wk-j/yaam/internal/config"
)

func TestSameLinkIgnoresTagsOnly(t *testing.T) {
	base := config.Connection{Host: "db", Port: 5432, Database: "shop", Role: "agent", SSLMode: "disable", AgentTimeoutSeconds: 30, Tags: []string{"prod"}}
	retagged := base
	retagged.Tags = []string{"dev", "shop"}
	if !sameLink(base, retagged) {
		t.Error("a Tag change rebuilds the pool")
	}
	moved := base
	moved.Host = "db2"
	if sameLink(base, moved) {
		t.Error("a host change keeps the old pool")
	}
}
