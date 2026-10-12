package main

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestStructuredOutputWireValues(t *testing.T) {
	a := testApp(t, nil)
	for _, tc := range []struct {
		name string
		code string
		want string
	}{
		{"defaults", "a", `{"status":"ok","valid":true,"options":{"layout":"dagre","theme_id":0,"sketch":false,"padding":100,"board_path":[],"center":false,"scale":1}}`},
		{"source settings", "vars: {d2-config: {pad: 0; dark-theme-id: 200; center: true}}\na", `{"status":"ok","valid":true,"options":{"layout":"dagre","theme_id":0,"dark_theme_id":200,"sketch":false,"padding":0,"board_path":[],"center":true,"scale":1}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := call(t, a, false, map[string]string{"code": tc.code})
			requireOK(t, result)
			var want any
			if err := json.Unmarshal([]byte(tc.want), &want); err != nil {
				t.Fatal(err)
			}
			if got := decodeOutput[any](t, result); !reflect.DeepEqual(got, want) {
				t.Fatalf("structured output changed: got %#v, want %#v", got, want)
			}
		})
	}
}
