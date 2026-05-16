package main

import "regexp"

// classAttrPattern matches the contents of a class="..." or class='...'
// attribute. Captures the inner string (without quotes).
var classAttrPattern = regexp.MustCompile(`(?i)class\s*=\s*["']([^"']*)["']`)

// classTokenPattern matches Litewind / Tailwind-style class identifiers.
// Permissive on purpose — false positives are harmless (no CSS rule will
// match a non-class token), false negatives break the page.
var classTokenPattern = regexp.MustCompile(`[A-Za-z_\-][\w:\-/]*`)

// ExtractClasses returns the set of class-name tokens referenced by
// class= attributes in the given content. Template-conditional fragments
// inside a class= attribute (e.g. `class="btn {% if active %}border-blue-500
// {% endif %}"`) tokenize through unharmed — the template-syntax tokens
// (`if`, `active`, ...) are extras that match nothing in CSS.
func ExtractClasses(content []byte) map[string]bool {
	classes := make(map[string]bool)
	for _, m := range classAttrPattern.FindAllSubmatch(content, -1) {
		if len(m) < 2 {
			continue
		}
		for _, tok := range classTokenPattern.FindAll(m[1], -1) {
			classes[string(tok)] = true
		}
	}
	return classes
}
