package main

import "strings"

// selectorClasses returns the class names referenced by a CSS selector.
// `.foo` → ["foo"]; `.foo.bar` → ["foo", "bar"]; `.foo .bar:hover`
// → ["foo", "bar"]; `body`, `:root`, `*` → []. CSS-escapes (`\:`, `\/`)
// are unescaped — Tailwind emits selectors like `.hover\:bg-red-500`
// for the class `hover:bg-red-500`, so escapes must round-trip back to
// the markup-side class identifier.
func selectorClasses(sel string) []string {
	var classes []string
	for i := 0; i < len(sel); i++ {
		switch sel[i] {
		case '.':
			end, name := scanClassName(sel, i+1)
			if name != "" {
				classes = append(classes, name)
			}
			i = end - 1 // -1 because the outer loop will i++
		case '[':
			// Skip attribute selector wholesale; classes inside attribute
			// values are not class identifiers.
			i = skipBalanced(sel, i, '[', ']')
		case '(':
			i = skipBalanced(sel, i, '(', ')')
		case '\\':
			// Top-level escape outside a class name (rare); skip one char.
			if i+1 < len(sel) {
				i++
			}
		}
	}
	return classes
}

// scanClassName scans a class identifier starting at start (just after
// the leading `.`). Returns the index after the class name and the
// decoded class name. CSS escapes inside the name (`\:`, `\/`, `\.`,
// generic `\xx`) are unescaped to their character form.
func scanClassName(s string, start int) (end int, name string) {
	var b strings.Builder
	i := start
	for i < len(s) {
		c := s[i]
		switch {
		case c == '\\' && i+1 < len(s):
			b.WriteByte(s[i+1])
			i += 2
		case isClassChar(c):
			b.WriteByte(c)
			i++
		default:
			return i, b.String()
		}
	}
	return i, b.String()
}

// isClassChar reports whether c can appear inside an unescaped CSS
// class name. Per CSS-syntax: alphanumerics, hyphen, underscore. Other
// characters (colon, slash, dot, etc.) must be backslash-escaped.
func isClassChar(c byte) bool {
	switch {
	case c >= 'A' && c <= 'Z':
		return true
	case c >= 'a' && c <= 'z':
		return true
	case c >= '0' && c <= '9':
		return true
	case c == '-' || c == '_':
		return true
	}
	return false
}

// skipBalanced returns the index of the matching closing delimiter for
// the open delimiter at start. If no match is found, returns len(s)-1
// (so the outer-loop i++ lands at len(s)).
func skipBalanced(s string, start int, open, close byte) int {
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return i
			}
		case '\\':
			if i+1 < len(s) {
				i++
			}
		}
	}
	return len(s) - 1
}

// keepRule reports whether a CSS rule with the given selector should
// survive purging given the set of classes referenced in content.
// Selectors with no class component (tag-only, pseudo-only, universal)
// are always kept. Selectors with classes are kept only when EVERY
// referenced class is present in the content set.
func keepRule(sel string, classes map[string]bool) bool {
	used := selectorClasses(sel)
	if len(used) == 0 {
		return true
	}
	for _, c := range used {
		if !classes[c] {
			return false
		}
	}
	return true
}

// splitSelectorList splits a CSS selector list (`.a, .b, .c`) on
// top-level commas. Commas inside `(...)` (functional pseudo-classes
// like `:is(...)`, `:where(...)`) or `[...]` (attribute selectors) are
// treated as literal and not split.
func splitSelectorList(list string) []string {
	var out []string
	depth := 0
	start := 0
	for i := 0; i < len(list); i++ {
		switch list[i] {
		case '(', '[':
			depth++
		case ')', ']':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(list[start:i]))
				start = i + 1
			}
		case '\\':
			if i+1 < len(list) {
				i++
			}
		}
	}
	out = append(out, strings.TrimSpace(list[start:]))
	return out
}
