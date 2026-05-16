package main

import (
	"bytes"
	"errors"
	"io"
	"maps"
	"strings"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
)

// Options controls Purge behavior.
type Options struct {
	// Keep is a list of class names to force-keep even if they do not
	// appear in the scanned content (e.g. classes injected by JS at runtime).
	Keep []string
}

// Purge filters cssData, keeping only rules whose every-selector class
// appears in classes (or in opts.Keep). Tag-only, pseudo-only, and
// universal selectors are always kept. @font-face, @keyframes, @page,
// @property, @counter-style, and other unknown at-rules are copied
// through verbatim. @media, @supports, @container, @layer are filtered
// recursively and the wrapper is dropped when no inner rule survives.
//
// Output is minified — no extra whitespace beyond what selectors
// require.
func Purge(cssData []byte, classes map[string]bool, opts Options) ([]byte, error) {
	effective := classes
	if len(opts.Keep) > 0 {
		effective = make(map[string]bool, len(classes)+len(opts.Keep))
		maps.Copy(effective, classes)
		for _, k := range opts.Keep {
			effective[k] = true
		}
	}

	var out bytes.Buffer
	p := css.NewParser(parse.NewInput(bytes.NewReader(cssData)), false)
	if _, err := walkBlock(p, effective, &out, false); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// walkBlock processes parser events until EOF (top level) or the
// matching closing brace (inside an at-rule body). Returns whether any
// output was emitted into out and any non-EOF parse error.
func walkBlock(p *css.Parser, classes map[string]bool, out *bytes.Buffer, insideAtRule bool) (bool, error) {
	emitted := false
	for {
		gt, _, data := p.Next()
		switch gt {
		case css.ErrorGrammar:
			err := p.Err()
			if errors.Is(err, io.EOF) {
				return emitted, nil
			}
			return emitted, err

		case css.CommentGrammar:
			// Drop comments entirely (minified output).

		case css.BeginRulesetGrammar:
			selectorBytes := concatTokens(p.Values())
			selectors := splitSelectorList(string(selectorBytes))
			kept := filterKeptSelectors(selectors, classes)
			if len(kept) == 0 {
				if err := skipRulesetBody(p); err != nil {
					return emitted, err
				}
				continue
			}
			out.WriteString(strings.Join(kept, ","))
			out.WriteByte('{')
			if err := emitDeclarations(p, out); err != nil {
				return emitted, err
			}
			out.WriteByte('}')
			emitted = true

		case css.EndRulesetGrammar:
			// Only relevant when an outer walker reaches a stray end
			// — normal ruleset bodies are drained by emitDeclarations
			// or skipRulesetBody.

		case css.BeginAtRuleGrammar:
			name := string(data)
			prelude := concatTokens(p.Values())
			if isFilterableAtRule(name) {
				var inner bytes.Buffer
				hadInner, err := walkBlock(p, classes, &inner, true)
				if err != nil {
					return emitted, err
				}
				if hadInner {
					out.WriteString(name)
					out.Write(prelude)
					out.WriteByte('{')
					out.Write(inner.Bytes())
					out.WriteByte('}')
					emitted = true
				}
				continue
			}
			out.WriteString(name)
			out.Write(prelude)
			out.WriteByte('{')
			if err := copyAtRuleBody(p, out); err != nil {
				return emitted, err
			}
			out.WriteByte('}')
			emitted = true

		case css.EndAtRuleGrammar:
			if !insideAtRule {
				continue
			}
			return emitted, nil

		case css.AtRuleGrammar:
			out.WriteString(string(data))
			out.Write(concatTokens(p.Values()))
			out.WriteByte(';')
			emitted = true

		case css.DeclarationGrammar, css.CustomPropertyGrammar:
			// Stray top-level declaration — unusual, but emit so we
			// don't silently drop data.
			writeDeclaration(out, data, p.Values())
			emitted = true

		case css.TokenGrammar, css.QualifiedRuleGrammar:
			// Stray top-level token / standalone qualified rule; ignore.
		}
	}
}

// emitDeclarations consumes parser events until EndRuleset and writes
// minified `prop:val;` declarations to out.
func emitDeclarations(p *css.Parser, out *bytes.Buffer) error {
	first := true
	for {
		gt, _, data := p.Next()
		switch gt {
		case css.ErrorGrammar:
			err := p.Err()
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		case css.DeclarationGrammar, css.CustomPropertyGrammar:
			if !first {
				out.WriteByte(';')
			}
			writeDeclaration(out, data, p.Values())
			first = false
		case css.CommentGrammar:
			// Skip.
		case css.EndRulesetGrammar:
			return nil
		case css.AtRuleGrammar, css.BeginAtRuleGrammar, css.EndAtRuleGrammar,
			css.BeginRulesetGrammar, css.QualifiedRuleGrammar, css.TokenGrammar:
			// Not expected inside a declaration block; skip.
		}
	}
}

// skipRulesetBody discards everything up to and including EndRuleset.
func skipRulesetBody(p *css.Parser) error {
	for {
		gt, _, _ := p.Next()
		switch gt {
		case css.ErrorGrammar:
			err := p.Err()
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		case css.EndRulesetGrammar:
			return nil
		case css.CommentGrammar, css.DeclarationGrammar, css.CustomPropertyGrammar,
			css.AtRuleGrammar, css.BeginAtRuleGrammar, css.EndAtRuleGrammar,
			css.BeginRulesetGrammar, css.QualifiedRuleGrammar, css.TokenGrammar:
			// Drain.
		}
	}
}

// copyAtRuleBody re-emits every event until the matching EndAtRule.
// Used for at-rules we keep verbatim (@font-face, @keyframes, @page,
// @property, @counter-style, etc.). Recursively descends into nested
// rulesets — keyframe selectors (`from`, `to`, `0%`, ...) are not
// classes and must not be class-filtered.
func copyAtRuleBody(p *css.Parser, out *bytes.Buffer) error {
	firstDecl := true
	for {
		gt, _, data := p.Next()
		switch gt {
		case css.ErrorGrammar:
			err := p.Err()
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err

		case css.EndAtRuleGrammar:
			return nil

		case css.BeginRulesetGrammar:
			firstDecl = true
			out.Write(concatTokens(p.Values()))
			out.WriteByte('{')
			if err := emitDeclarations(p, out); err != nil {
				return err
			}
			out.WriteByte('}')

		case css.DeclarationGrammar, css.CustomPropertyGrammar:
			if !firstDecl {
				out.WriteByte(';')
			}
			writeDeclaration(out, data, p.Values())
			firstDecl = false

		case css.AtRuleGrammar:
			out.WriteString(string(data))
			out.Write(concatTokens(p.Values()))
			out.WriteByte(';')

		case css.BeginAtRuleGrammar:
			out.WriteString(string(data))
			out.Write(concatTokens(p.Values()))
			out.WriteByte('{')
			if err := copyAtRuleBody(p, out); err != nil {
				return err
			}
			out.WriteByte('}')

		case css.CommentGrammar:
			// Skip.
		case css.EndRulesetGrammar, css.QualifiedRuleGrammar, css.TokenGrammar:
			// Not expected at this level inside an at-rule body; skip.
		}
	}
}

// writeDeclaration writes `prop:val`. The caller controls the trailing
// semicolon so it can suppress one on the final declaration of a block.
// Leading and trailing whitespace tokens are trimmed for minification;
// internal whitespace stays (multi-value properties like `padding:4px
// 8px` need it).
func writeDeclaration(out *bytes.Buffer, prop []byte, values []css.Token) {
	out.Write(prop)
	out.WriteByte(':')
	out.Write(bytes.TrimSpace(concatTokens(values)))
}

// concatTokens joins token Data verbatim. Whitespace tokens preserved by
// the parser carry the separators that the original source had — we
// keep them so descendant combinators (`.outer .inner`) and other
// space-sensitive selectors survive intact.
func concatTokens(tokens []css.Token) []byte {
	if len(tokens) == 0 {
		return nil
	}
	var b bytes.Buffer
	for _, t := range tokens {
		b.Write(t.Data)
	}
	return b.Bytes()
}

// filterKeptSelectors returns the subset of selectors whose class set
// is fully present in classes. Tag/pseudo-only selectors are always
// kept (see keepRule).
func filterKeptSelectors(selectors []string, classes map[string]bool) []string {
	out := make([]string, 0, len(selectors))
	for _, s := range selectors {
		if keepRule(s, classes) {
			out = append(out, s)
		}
	}
	return out
}

// isFilterableAtRule reports whether the at-rule body should be class-
// filtered (true) or copied through verbatim (false). Filterable at-
// rules wrap normal rulesets; copy-through at-rules either have no
// selectors (@font-face, @charset, @property, @counter-style) or have
// non-class selectors (@keyframes selectors are `from`/`to`/`0%`).
func isFilterableAtRule(name string) bool {
	switch strings.ToLower(name) {
	case "@media", "@supports", "@container", "@layer", "@scope":
		return true
	}
	return false
}
