package main

import (
	"errors"
	"regexp"
	"strings"
)

var cssFunction = regexp.MustCompile(`([a-zA-Z_-][a-zA-Z0-9_-]*)\(`)
var cssAtRule = regexp.MustCompile(`@([a-zA-Z-]+)`)
var cssFragment = regexp.MustCompile(`^#[a-zA-Z0-9_.:-]+$`)

// The renderer owns the stylesheet, but LaTeX can contribute paint and style
// values. Admit the generated CSS subset, not arbitrary CSS supplied by a label.
// In particular image-set("URL") can fetch without a url() token, and CSS
// escapes/comments can disguise identifiers. Reject those forms explicitly.
func checkCSS(value string, stylesheet bool) error {
	if strings.Contains(value, "\\") || strings.Contains(value, "/*") || strings.Contains(value, "*/") {
		return errors.New("escaped or commented SVG CSS is unsupported")
	}
	for _, match := range cssAtRule.FindAllStringSubmatch(value, -1) {
		if !stylesheet {
			return errors.New("SVG inline CSS at-rules are unsupported")
		}
		switch strings.ToLower(match[1]) {
		case "font-face", "media", "keyframes":
		default:
			return errors.New("SVG CSS at-rule is unsupported")
		}
	}
	for _, match := range cssFunction.FindAllStringSubmatchIndex(value, -1) {
		name := strings.ToLower(value[match[2]:match[3]])
		switch name {
		case "url":
			rest := value[match[1]:]
			end := strings.IndexByte(rest, ')')
			if end < 0 {
				return errors.New("invalid SVG CSS URL")
			}
			target := strings.TrimSpace(rest[:end])
			if len(target) >= 2 && (target[0] == '\'' || target[0] == '"') && target[len(target)-1] == target[0] {
				target = target[1 : len(target)-1]
			}
			if cssFragment.MatchString(target) {
				continue
			}
			if stylesheet && strings.HasPrefix(target, "data:application/font-woff;base64,") {
				continue
			}
			return errors.New("external SVG CSS resources are disabled")
		case "rgb", "rgba", "hsl", "hsla", "hwb", "lab", "lch", "oklab", "oklch", "color", "color-mix",
			"var", "calc", "min", "max", "clamp", "translate", "translatex", "translatey", "scale", "scalex", "scaley", "rotate", "matrix",
			"cubic-bezier", "steps", "not", "is", "where", "drop-shadow":
		default:
			return errors.New("SVG CSS function is unsupported")
		}
	}
	return nil
}
