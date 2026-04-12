package ai

import (
	"regexp"
	"strings"
)

// allowedTags is the set of HTML tags allowed in AI-generated content.
var allowedTags = map[string]bool{
	"p": true, "h1": true, "h2": true, "h3": true,
	"ul": true, "ol": true, "li": true,
	"strong": true, "em": true, "b": true, "i": true,
	"code": true, "pre": true,
	"a": true, "br": true, "blockquote": true,
	"table": true, "thead": true, "tbody": true, "tr": true, "th": true, "td": true,
	"hr": true, "span": true,
}

// allowedAttrs is the set of attributes allowed per tag.
var allowedAttrs = map[string]map[string]bool{
	"a": {"href": true, "title": true},
}

var (
	tagPattern   = regexp.MustCompile(`<(/?)([a-zA-Z][a-zA-Z0-9]*)\b([^>]*)(/?)>`)
	eventPattern = regexp.MustCompile(`(?i)\bon[a-z]+\s*=`)
	stylePattern = regexp.MustCompile(`(?i)\bstyle\s*=`)
)

// SanitizeHTML strips disallowed tags and attributes from HTML content.
func SanitizeHTML(html string) string {
	return tagPattern.ReplaceAllStringFunc(html, func(tag string) string {
		m := tagPattern.FindStringSubmatch(tag)
		if m == nil {
			return ""
		}
		tagName := strings.ToLower(m[2])
		if !allowedTags[tagName] {
			return ""
		}

		attrs := m[3]
		// Strip event handlers and style attributes
		if eventPattern.MatchString(attrs) || stylePattern.MatchString(attrs) {
			attrs = eventPattern.ReplaceAllString(attrs, "")
			attrs = stylePattern.ReplaceAllString(attrs, "")
		}

		// For tags with allowed attrs, keep only those
		if allowed, ok := allowedAttrs[tagName]; ok {
			attrs = filterAttrs(attrs, allowed)
		} else {
			attrs = "" // no attrs allowed
		}

		closing := m[1]
		selfClosing := m[4]
		if attrs != "" {
			return "<" + closing + tagName + " " + strings.TrimSpace(attrs) + selfClosing + ">"
		}
		return "<" + closing + tagName + selfClosing + ">"
	})
}

func filterAttrs(attrs string, allowed map[string]bool) string {
	attrPattern := regexp.MustCompile(`([a-zA-Z-]+)\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	matches := attrPattern.FindAllStringSubmatch(attrs, -1)
	var kept []string
	for _, m := range matches {
		name := strings.ToLower(m[1])
		if allowed[name] {
			value := m[2]
			if value == "" {
				value = m[3]
			}
			// Block javascript: URLs
			if name == "href" && strings.HasPrefix(strings.TrimSpace(strings.ToLower(value)), "javascript:") {
				continue
			}
			kept = append(kept, name+`="`+value+`"`)
		}
	}
	return strings.Join(kept, " ")
}
