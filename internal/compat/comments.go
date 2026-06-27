package compat

import (
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// maskUnit is the building block for the placeholder delimiter that stands in
// for a `{{...}}` action span while HTML comments are stripped. It is a Unicode
// private-use-area rune, which the HTML tokenizer treats as ordinary text.
// StripHTMLComments repeats it until the resulting delimiter is absent from the
// source, so placeholders can never collide with literal template text.
const maskUnit = "\ue000"

// StripHTMLComments removes HTML comments (`<!-- ... -->`) from a template
// source string, matching the comment-stripping that html/template performs
// during its escape pass. Like NormalizeTemplateSpacing, it is a pre-parse
// source transform applied before the template is parsed.
//
// LiveTemplate builds its static segments by walking the raw parse tree, which
// never triggers html/template's escape pass — so without this, developer/
// internal HTML comments survive verbatim into the statics and ship to the
// client (visible in view-source). Template comments (`{{/* ... */}}`) are
// already removed at parse time and are unaffected.
//
// `{{...}}` action spans are masked out before stripping so that a literal
// `<!--` appearing *inside* an action (e.g. `{{"<!--"}}`, a `{{/* <!-- */}}`
// template comment, or a `"<!--"` string argument) is never mistaken for the
// start of an HTML comment — matching html/template, which never strips inside
// an action. The masking scanner respects Go template lexing (string, raw
// string, char literals, and `/* */` comments) so the closing `}}` is found
// correctly even when `}}` appears inside a quote or comment.
//
// Stripping uses the x/net/html tokenizer (rather than a regex) on the masked
// string, so it is context-aware:
//   - comment-like text inside attribute values (e.g.
//     `title="<!-- not a comment -->"`) is preserved, not mistaken for a comment;
//   - comment markup inside RAWTEXT/RCDATA elements (`<script>`, `<style>`,
//     `<textarea>`) is left verbatim, since the tokenizer does not treat it as a
//     comment there.
//
// A comment that wraps an action (`<!-- {{.X}} -->`) is removed in full,
// action included — matching html/template.
//
// Residual divergences from html/template (both unchanged from prior behavior,
// i.e. not regressions): in `<script>` context html/template rejects `<!--`
// entirely, and in `<textarea>`/RCDATA context it escapes `<!--` to `&lt;!--`;
// here both are preserved verbatim.
func StripHTMLComments(s string) string {
	// Fast path: no comment markers anywhere means nothing to strip.
	if !strings.Contains(s, "<!--") {
		return s
	}

	// Grow the placeholder delimiter until it is absent from the source, so a
	// masked action can never be confused with literal text on restore.
	mask := maskUnit
	for strings.Contains(s, mask) {
		mask += maskUnit
	}

	masked, actions := maskActions(s, mask)
	if !strings.Contains(masked, "<!--") {
		// Every `<!--` was inside a `{{...}}` action, so there is no real HTML
		// comment to strip. Return the source untouched.
		return s
	}

	stripped := stripCommentTokens(masked)
	return restoreActions(stripped, actions, mask)
}

// stripCommentTokens rebuilds the input from tokenizer Raw() output, dropping
// only CommentTokens. RAWTEXT/RCDATA comment-like content is not tokenized as a
// comment, so it is left verbatim by construction.
func stripCommentTokens(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	z := html.NewTokenizer(strings.NewReader(s))
	for {
		// strings.Reader never returns a read error, so ErrorToken here only
		// signals EOF — never a mid-stream tokenization failure that would
		// truncate output.
		switch z.Next() {
		case html.ErrorToken:
			return b.String()
		case html.CommentToken:
		default:
			b.Write(z.Raw())
		}
	}
}

// maskActions replaces every {{...}} span with mask+index+mask;
// restoreActions reverses it.
func maskActions(s, mask string) (string, []string) {
	var b strings.Builder
	b.Grow(len(s))
	var actions []string
	i := 0
	for i < len(s) {
		start := strings.Index(s[i:], "{{")
		if start < 0 {
			b.WriteString(s[i:])
			break
		}
		start += i
		b.WriteString(s[i:start])

		end := actionEnd(s, start)
		if end < 0 {
			// Unterminated action: emit the remainder verbatim and let the
			// template parser report the error downstream.
			b.WriteString(s[start:])
			break
		}

		b.WriteString(mask)
		b.WriteString(strconv.Itoa(len(actions)))
		b.WriteString(mask)
		actions = append(actions, s[start:end])
		i = end
	}
	return b.String(), actions
}

// restoreActions reinstates the original action spans. Placeholders that were
// inside a stripped comment are simply absent, so their actions drop out too —
// matching html/template's removal of a comment-wrapped action.
func restoreActions(s string, actions []string, mask string) string {
	if len(actions) == 0 {
		return s
	}
	pairs := make([]string, 0, len(actions)*2)
	for i, action := range actions {
		pairs = append(pairs, mask+strconv.Itoa(i)+mask, action)
	}
	return strings.NewReplacer(pairs...).Replace(s)
}

// actionEnd returns the index just past the `}}` that closes the action
// beginning at start (where s[start:] begins with "{{"), or -1 if unterminated.
// It skips over string, raw-string, char literals, and `/* */` comments so a
// `}}` inside any of them does not end the action prematurely.
func actionEnd(s string, start int) int {
	for i := start + 2; i < len(s); {
		switch {
		case strings.HasPrefix(s[i:], "}}"):
			return i + 2
		case s[i] == '"':
			i = skipQuoted(s, i, '"')
		case s[i] == '\'':
			i = skipQuoted(s, i, '\'')
		case s[i] == '`':
			i = skipRawString(s, i)
		case strings.HasPrefix(s[i:], "/*"):
			i = skipActionComment(s, i)
		default:
			i++
		}
		if i < 0 {
			return -1
		}
	}
	return -1
}

// skipQuoted returns the index just past the closing quote q (with backslash
// escapes), where s[i] == q opens the literal, or -1 if unterminated.
func skipQuoted(s string, i int, q byte) int {
	for i++; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++ // skip the escaped byte
			if i >= len(s) {
				return -1 // trailing backslash: unterminated
			}
		case q:
			return i + 1
		}
	}
	return -1
}

// skipRawString returns the index just past the closing backtick (raw strings
// have no escapes), where s[i] == '`', or -1 if unterminated.
func skipRawString(s string, i int) int {
	for i++; i < len(s); i++ {
		if s[i] == '`' {
			return i + 1
		}
	}
	return -1
}

// skipActionComment returns the index just past the `*/` closing the action
// comment opened by `/*` at i, or -1 if unterminated.
func skipActionComment(s string, i int) int {
	if end := strings.Index(s[i+2:], "*/"); end >= 0 {
		return i + 2 + end + 2
	}
	return -1
}
