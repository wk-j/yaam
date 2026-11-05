// Package classify decides, from statement types alone, which statements of
// an Agent Request are Reads (ADR 0004) and which start or end a transaction
// (ADR 0005). It uses PostgreSQL's own parser through pg_query_go.
package classify

import (
	"encoding/json"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

type Kind string

const (
	Read      Kind = "read"
	Write     Kind = "write"
	TxControl Kind = "txcontrol"
)

// Statement is one top-level statement with its own text.
type Statement struct {
	Text string `json:"text"`
	Kind Kind   `json:"kind"`
}

// Parse splits sql into classified statements. A parse failure returns the
// parser's message as the error.
func Parse(sql string) ([]Statement, error) {
	tree, err := pg_query.ParseToJSON(sql)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Stmts []struct {
			Stmt     map[string]any `json:"stmt"`
			Location int            `json:"stmt_location"`
			Len      int            `json:"stmt_len"`
		} `json:"stmts"`
	}
	if err := json.Unmarshal([]byte(tree), &parsed); err != nil {
		return nil, err
	}
	out := make([]Statement, 0, len(parsed.Stmts))
	for _, s := range parsed.Stmts {
		end := len(sql)
		if s.Len > 0 {
			end = s.Location + s.Len
		}
		out = append(out, Statement{Text: strings.TrimSpace(sql[s.Location:end]), Kind: kindOf(s.Stmt)})
	}
	return out, nil
}

func kindOf(stmt map[string]any) Kind {
	for typ, body := range stmt { // exactly one key: the node type
		node, _ := body.(map[string]any)
		switch typ {
		case "SelectStmt":
			if modifies(node) {
				return Write
			}
			return Read
		case "VariableShowStmt":
			return Read
		case "ExplainStmt":
			if hasAnalyze(node) {
				return Write
			}
			return Read
		case "TransactionStmt":
			switch node["kind"] {
			case "TRANS_STMT_BEGIN", "TRANS_STMT_START", "TRANS_STMT_COMMIT", "TRANS_STMT_ROLLBACK", "TRANS_STMT_PREPARE":
				return TxControl
			}
		}
	}
	return Write
}

// modifies reports whether a SELECT tree holds a locking clause, an INTO
// clause or a data-modifying statement (in a WITH) anywhere.
func modifies(v any) bool {
	switch v := v.(type) {
	case map[string]any:
		for k, child := range v {
			switch k {
			case "InsertStmt", "UpdateStmt", "DeleteStmt", "MergeStmt":
				return true
			case "intoClause":
				if child != nil {
					return true
				}
			case "lockingClause":
				if list, ok := child.([]any); ok && len(list) > 0 {
					return true
				}
			}
			if modifies(child) {
				return true
			}
		}
	case []any:
		for _, child := range v {
			if modifies(child) {
				return true
			}
		}
	}
	return false
}

func hasAnalyze(explain map[string]any) bool {
	opts, _ := explain["options"].([]any)
	for _, o := range opts {
		def, _ := o.(map[string]any)["DefElem"].(map[string]any)
		if name, _ := def["defname"].(string); strings.EqualFold(name, "analyze") {
			return true
		}
	}
	return false
}
