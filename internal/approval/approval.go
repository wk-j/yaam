// Package approval holds the Requests that wait for the User's Approval
// (ADR 0003): the Agent's tool call blocks in Wait until the User decides
// or the call goes away.
package approval

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"

	"github.com/wk-j/yaam/internal/classify"
)

// ErrNotFound means no pending item has that id (decided or cancelled).
var ErrNotFound = errors.New("no such pending Request; it was decided or cancelled")

// Pending is a Request waiting for Approval, as shown in the web UI.
type Pending struct {
	ID         string               `json:"id"`
	Time       string               `json:"time"`
	AgentName  string               `json:"agentName"`
	Project    string               `json:"project"`
	Connection string               `json:"connection"`
	Purpose    string               `json:"purpose"`
	SQL        string               `json:"sql"`
	Statements []classify.Statement `json:"statements"`
}

type item struct {
	p        Pending
	decision chan bool // true = approved
}

// Broker holds the pending items in arrival order.
type Broker struct {
	mu       sync.Mutex
	items    []*item
	onChange func() // called after an item is added or removed
}

func NewBroker(onChange func()) *Broker {
	return &Broker{onChange: onChange}
}

func NewID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "r_" + hex.EncodeToString(b)
}

// Wait adds p (its ID must be set) and blocks until the User approves
// (true) or rejects (false). If ctx ends first, the item is removed and
// ctx's error is returned: the Request must then never run.
func (b *Broker) Wait(ctx context.Context, p Pending) (bool, error) {
	it := &item{p: p, decision: make(chan bool, 1)}
	b.mu.Lock()
	b.items = append(b.items, it)
	b.mu.Unlock()
	b.onChange()
	select {
	case approved := <-it.decision:
		return approved, nil
	case <-ctx.Done():
		if b.remove(p.ID) != nil {
			b.onChange()
		}
		return false, ctx.Err()
	}
}

// Decide answers the item id; ErrNotFound when it is gone.
func (b *Broker) Decide(id string, approve bool) error {
	it := b.remove(id)
	if it == nil {
		return ErrNotFound
	}
	it.decision <- approve
	b.onChange()
	return nil
}

func (b *Broker) remove(id string) *item {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, it := range b.items {
		if it.p.ID == id {
			b.items = append(b.items[:i], b.items[i+1:]...)
			return it
		}
	}
	return nil
}

// List returns the pending items, oldest first.
func (b *Broker) List() []Pending {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Pending, 0, len(b.items))
	for _, it := range b.items {
		out = append(out, it.p)
	}
	return out
}
