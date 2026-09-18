// Package jsonc contains the small, dependency-free JSONC normalizer used by affer.
// It intentionally keeps the public surface compatible with the upstream package
// while making the parser usable in an offline build.
package jsonc

import (
	"bytes"
	"fmt"
)

// Parse strips comments and trailing commas from JSONC input. Strings, escaped
// quotes, and escaped backslashes are handled by the scanner rather than by
// regular expressions, so URLs and source snippets remain intact.
func Parse(src []byte) ([]byte, error) {
	var out bytes.Buffer
	out.Grow(len(src))
	inString := false
	escaped := false
	for i := 0; i < len(src); i++ {
		c := src[i]
		if inString {
			out.WriteByte(c)
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
			out.WriteByte(c)
		case '/':
			if i+1 >= len(src) {
				out.WriteByte(c)
				continue
			}
			switch src[i+1] {
			case '/':
				i += 2
				for i < len(src) && src[i] != '\n' && src[i] != '\r' { i++ }
				if i < len(src) { out.WriteByte(src[i]) }
			case '*':
				i += 2
				closed := false
				for i+1 < len(src) {
					if src[i] == '*' && src[i+1] == '/' { i++; closed = true; break }
					i++
				}
				if !closed { return nil, fmt.Errorf("unterminated JSONC block comment") }
			default:
				out.WriteByte(c)
			}
		default:
			out.WriteByte(c)
		}
	}
	if inString || escaped { return nil, fmt.Errorf("unterminated JSONC string") }

	// Remove commas followed only by whitespace and a closing array/object.
	clean := out.Bytes()
	var normalized bytes.Buffer
	normalized.Grow(len(clean))
	inString, escaped = false, false
	for i := 0; i < len(clean); i++ {
		c := clean[i]
		if inString {
			normalized.WriteByte(c)
			if escaped { escaped = false } else if c == '\\' { escaped = true } else if c == '"' { inString = false }
			continue
		}
		if c == '"' { inString = true; normalized.WriteByte(c); continue }
		if c == ',' {
			j := i + 1
			for j < len(clean) && (clean[j] == ' ' || clean[j] == '\t' || clean[j] == '\r' || clean[j] == '\n') { j++ }
			if j < len(clean) && (clean[j] == '}' || clean[j] == ']') { continue }
		}
		normalized.WriteByte(c)
	}
	return normalized.Bytes(), nil
}
