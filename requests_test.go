package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestLegacyRequestAliases(t *testing.T) {
	a := testApp(t, nil)
	for _, modes := range [][2]string{{"standard", "ascii"}, {"extended", "unicode"}} {
		canonical := call(t, a, true, map[string]any{"code": "a -> b", "format": "ascii", "ascii_mode": modes[0]})
		alias := call(t, a, true, map[string]any{"code": "a -> b", "format": "ASCII", "ascii_mode": strings.ToUpper(modes[1])})
		requireOK(t, canonical)
		requireOK(t, alias)
		if !reflect.DeepEqual(decodeOutput[renderOutput](t, canonical), decodeOutput[renderOutput](t, alias)) {
			t.Fatalf("alias %q changed the artifact or options", modes[1])
		}
	}
}

func TestRequestErrorPriority(t *testing.T) {
	for _, tc := range []struct {
		name, input, code, message string
		render, busy               bool
	}{
		{"compile before format", `{"format":"bad","ascii_mode":"bad"}`, "invalid_arguments", "format, ascii_mode, output_name, and overwrite apply only to render-d2", false, false},
		{"format before mode", `{"format":"bad","ascii_mode":"bad"}`, "invalid_arguments", "format must be png, svg, or ascii", true, false},
		{"mode before output", `{"ascii_mode":"bad","output_name":"a.svg"}`, "invalid_arguments", "ascii_mode must be extended or standard", true, false},
		{"output before source", `{"output_name":"a.svg"}`, "file_access_denied", "output_name requires --output-dir", true, false},
		{"format before busy", `{"format":"bad"}`, "invalid_arguments", "format must be png, svg, or ascii", true, true},
		{"busy before source", `{}`, "busy", "renderer is busy; retry later", true, true},
		{"busy before layout", `{"code":"a","layout":"bad"}`, "busy", "renderer is busy; retry later", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := testApp(t, nil)
			if tc.busy {
				for range cap(a.slots) {
					a.slots <- struct{}{}
				}
			}
			result := call(t, a, tc.render, json.RawMessage(tc.input))
			requireError(t, result, tc.code)
			if got := decodeOutput[errorOutput](t, result).Error.Message; got != tc.message {
				t.Fatalf("message=%q, want %q", got, tc.message)
			}
		})
	}
}
