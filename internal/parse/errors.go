package parse

import (
	"fmt"
	"strings"
)

// ParseError provides structured error information from template parsing and tree building.
type ParseError struct {
	Phase    string // "parse", "eval", "build"
	NodeType string // "action", "if", "range", "with", "list", "pipe", "field", etc.
	Expr     string // The expression being evaluated (if applicable)
	Pos      int    // Node position in source (from parse.Node.Position())
	Msg      string // Human-readable message
	Err      error  // Underlying cause
}

func (e *ParseError) Error() string {
	var b strings.Builder
	b.WriteString(e.Phase)
	if e.NodeType != "" {
		fmt.Fprintf(&b, " [%s]", e.NodeType)
	}
	if e.Pos > 0 {
		fmt.Fprintf(&b, " at position %d", e.Pos)
	}
	if e.Msg != "" {
		b.WriteString(": ")
		b.WriteString(e.Msg)
	}
	if e.Err != nil {
		if e.Msg != "" {
			fmt.Fprintf(&b, ": %v", e.Err)
		} else {
			b.WriteString(": ")
			b.WriteString(e.Err.Error())
		}
	}
	return b.String()
}

func (e *ParseError) Unwrap() error { return e.Err }
