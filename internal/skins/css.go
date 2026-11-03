package skins

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
)

// allowedAtRules is an allowlist. Anything that can load another document,
// execute code or redefine the document namespace is rejected.
var allowedAtRules = map[string]bool{
	"@charset":           true,
	"@container":         true,
	"@font-face":         true,
	"@keyframes":         true,
	"@-webkit-keyframes": true,
	"@layer":             true,
	"@media":             true,
	"@property":          true,
	"@starting-style":    true,
	"@supports":          true,
}

var (
	cssSchemePattern  = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.\-]*:`)
	cssURLPattern     = regexp.MustCompile(`(?i)url\s*\(([^)]*)\)`)
	cssCommentPattern = regexp.MustCompile(`(?s)/\*.*?\*/`)
	cssExpression     = regexp.MustCompile(`expression\s*\(`)
)

// ValidateCSS parses a skin stylesheet and returns every local asset path it
// references. Remote URLs, data URLs, path traversal, @import and executable
// CSS constructs are rejected. The returned paths are relative to the
// stylesheet's directory and use forward slashes.
func ValidateCSS(source string) ([]string, error) {
	if len(source) > maxCSSSize {
		return nil, validationError("stylesheet exceeds the 1 MB limit")
	}
	if !utf8.ValidString(source) {
		return nil, validationError("stylesheet is not valid UTF-8")
	}

	// Scan the raw stylesheet before parsing. Some dangerous constructs (for
	// example a javascript: URL) make the CSS parser fail before we can inspect
	// the token stream, and the raw scan also catches escapes inside custom
	// property values.
	rawCSS := cssCommentPattern.ReplaceAllString(source, "")
	if strings.Contains(strings.ToLower(decodeCSSEscapes(rawCSS)), "@import") {
		return nil, validationError("@import is not allowed in skin stylesheets")
	}
	if err := rejectDangerousText(rawCSS); err != nil {
		return nil, err
	}

	assets := make(map[string]bool)
	parser := css.NewParser(parse.NewInput(strings.NewReader(source)), false)

	for {
		grammar, token, data := parser.Next()
		if grammar == css.ErrorGrammar {
			if err := parser.Err(); err != nil && !errors.Is(err, io.EOF) {
				return nil, validationError("stylesheet is not valid CSS: " + err.Error())
			}
			break
		}

		if grammar == css.AtRuleGrammar || grammar == css.BeginAtRuleGrammar {
			name := strings.ToLower(string(data))
			if !allowedAtRules[name] {
				if name == "@import" {
					return nil, validationError("@import is not allowed in skin stylesheets")
				}
				return nil, validationError(fmt.Sprintf("at-rule %s is not allowed", name))
			}
		}

		if grammar == css.DeclarationGrammar || grammar == css.CustomPropertyGrammar {
			property := strings.ToLower(string(data))
			if property == "behavior" || property == "-moz-binding" {
				return nil, validationError(fmt.Sprintf("CSS property %s is not allowed", property))
			}
		}

		if token == css.BadURLToken || token == css.BadStringToken {
			return nil, validationError("stylesheet contains an invalid URL or string")
		}

		for _, value := range parser.Values() {
			switch value.TokenType {
			case css.URLToken:
				raw, err := urlTokenValue(string(value.Data))
				if err != nil {
					return nil, err
				}
				asset, err := localAssetPath(raw)
				if err != nil {
					return nil, err
				}
				assets[asset] = true
			case css.BadURLToken, css.BadStringToken:
				return nil, validationError("stylesheet contains an invalid URL or string")
			case css.FunctionToken:
				function := strings.ToLower(string(value.Data))
				switch function {
				case "expression(", "-moz-binding(", "image-set(", "-webkit-image-set(", "image(", "-webkit-image(", "src(", "cross-fade(", "element(", "paint(":
					return nil, validationError(fmt.Sprintf("CSS function %s is not allowed", strings.TrimSuffix(function, "(")))
				}
			}

			if err := rejectDangerousText(string(value.Data)); err != nil {
				return nil, err
			}

			// Custom properties are returned as one opaque value token. Scan
			// that value for URLs so a skin cannot hide a remote request in
			// --background: url(https://example.test/x.png).
			if value.TokenType == css.CustomPropertyValueToken {
				if err := collectURLsFromCustomProperty(string(value.Data), assets); err != nil {
					return nil, err
				}
			}
		}

		// @import is also rejected when the parser reports it as a bare
		// at-rule without values.
		if token == css.AtKeywordToken && strings.EqualFold(string(data), "@import") {
			return nil, validationError("@import is not allowed in skin stylesheets")
		}
	}

	result := make([]string, 0, len(assets))
	for asset := range assets {
		result = append(result, asset)
	}
	sort.Strings(result)
	return result, nil
}

func collectURLsFromCustomProperty(value string, assets map[string]bool) error {
	cleaned := cssCommentPattern.ReplaceAllString(value, "")
	for _, match := range cssURLPattern.FindAllStringSubmatch(cleaned, -1) {
		if len(match) < 2 {
			continue
		}
		raw := strings.TrimSpace(match[1])
		if len(raw) >= 2 && ((raw[0] == '"' && raw[len(raw)-1] == '"') || (raw[0] == '\'' && raw[len(raw)-1] == '\'')) {
			raw = raw[1 : len(raw)-1]
		}
		asset, err := localAssetPath(decodeCSSEscapes(raw))
		if err != nil {
			return err
		}
		assets[asset] = true
	}
	return nil
}

func rejectDangerousText(value string) error {
	lower := strings.ToLower(decodeCSSEscapes(value))
	if strings.Contains(lower, "javascript:") || strings.Contains(lower, "vbscript:") {
		return validationError("stylesheet contains a script URL")
	}
	if strings.Contains(lower, "-moz-binding") {
		return validationError("stylesheet contains a forbidden binding")
	}
	if cssExpression.MatchString(lower) {
		return validationError("stylesheet contains a forbidden CSS expression")
	}
	return nil
}

func urlTokenValue(token string) (string, error) {
	trimmed := strings.TrimSpace(token)
	if len(trimmed) < 5 || !strings.EqualFold(trimmed[:4], "url(") || trimmed[len(trimmed)-1] != ')' {
		return "", validationError("stylesheet contains an invalid URL token")
	}
	inner := strings.TrimSpace(trimmed[4 : len(trimmed)-1])
	if len(inner) >= 2 {
		first, last := inner[0], inner[len(inner)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			inner = inner[1 : len(inner)-1]
		}
	}
	if inner == "" {
		return "", validationError("stylesheet contains an empty URL")
	}
	return decodeCSSEscapes(inner), nil
}

func localAssetPath(raw string) (string, error) {
	if raw == "" {
		return "", validationError("stylesheet contains an empty URL")
	}
	for _, r := range raw {
		if r == 0 || r < 0x20 || r == 0x7f {
			return "", validationError("stylesheet contains a control character in a URL")
		}
	}
	if strings.Contains(raw, "\\") {
		return "", validationError("backslashes are not allowed in skin URLs")
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "//") || strings.HasPrefix(raw, "/") || cssSchemePattern.MatchString(raw) {
		return "", validationError(fmt.Sprintf("remote or absolute URL is not allowed: %s", raw))
	}
	if strings.ContainsAny(raw, "?#") {
		return "", validationError(fmt.Sprintf("query strings and fragments are not allowed in skin URLs: %s", raw))
	}

	decoded, err := url.PathUnescape(raw)
	if err != nil {
		return "", validationError(fmt.Sprintf("invalid URL encoding: %s", raw))
	}
	if strings.ContainsAny(decoded, "\\?#") || strings.HasPrefix(decoded, "/") {
		return "", validationError(fmt.Sprintf("unsafe skin URL: %s", raw))
	}
	cleaned := path.Clean(decoded)
	if cleaned != decoded || cleaned == "." || cleaned == ".." {
		return "", validationError(fmt.Sprintf("unsafe skin URL: %s", raw))
	}
	if !isAssetExtension(strings.ToLower(path.Ext(cleaned))) {
		return "", validationError(fmt.Sprintf("stylesheet references an unsupported asset type: %s", raw))
	}
	return cleaned, nil
}

func decodeCSSEscapes(input string) string {
	if !strings.Contains(input, "\\") {
		return input
	}
	var builder strings.Builder
	builder.Grow(len(input))
	for index := 0; index < len(input); {
		if input[index] != '\\' {
			builder.WriteByte(input[index])
			index++
			continue
		}
		index++
		if index >= len(input) {
			break
		}
		switch input[index] {
		case '\n':
			index++
			continue
		case '\r':
			index++
			if index < len(input) && input[index] == '\n' {
				index++
			}
			continue
		}

		start := index
		for index < len(input) && index-start < 6 && isHexDigit(input[index]) {
			index++
		}
		if index > start {
			value, err := strconv.ParseInt(input[start:index], 16, 32)
			if err == nil {
				builder.WriteRune(rune(value))
			}
			if index < len(input) && isCSSWhitespace(input[index]) {
				index++
			}
			continue
		}
		builder.WriteByte(input[index])
		index++
	}
	return builder.String()
}

func isHexDigit(value byte) bool {
	return (value >= '0' && value <= '9') || (value >= 'a' && value <= 'f') || (value >= 'A' && value <= 'F')
}

func isCSSWhitespace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\n' || value == '\r' || value == '\f'
}
