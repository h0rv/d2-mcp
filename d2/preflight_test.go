package d2

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/d2lang/d2/d2compiler"
	"github.com/d2lang/d2/d2parser"
)

func TestPreflightRejectsCompileAmplification(t *testing.T) {
	cases := []struct {
		name, code, want string
		files            fstest.MapFS
	}{
		{"nested maps", strings.Repeat("a: {", 200) + "z" + strings.Repeat("}", 200), "limit_exceeded", nil},
		{"quoted block spoof in label", "a: value@|tag " + strings.Repeat("b: {", 20) + "z" + strings.Repeat("}", 20) + " |", "limit_exceeded", nil},
		{"bare import block spoof in key", "@|tag " + strings.Repeat("b: {", 20) + "z" + strings.Repeat("}", 20) + " |", "limit_exceeded", nil},
		{"embedded parenthesis block spoof", "a(|tag " + strings.Repeat("b: {", 20) + "z" + strings.Repeat("}", 20) + " |", "limit_exceeded", nil},
		{"embedded closing parenthesis block spoof", "a)|tag " + strings.Repeat("b: {", 20) + "z" + strings.Repeat("}", 20) + " |", "limit_exceeded", nil},
		{"dotted depth", strings.Repeat("a.", 17) + "z", "limit_exceeded", nil},
		{"composed dotted depth", "a.b.c.d.e.f.g.h: {i.j.k.l.m.n.o.p.q}", "limit_exceeded", nil},
		{"wildcard edges", "a; b; c\n* -> *", "unsupported_feature", nil},
		{"wildcard children", "a; b\n*.child: child", "unsupported_feature", nil},
		{"wildcard children between globs", "a; b\n*.child.*.style.fill: red", "unsupported_feature", nil},
		{"quoted wildcard attribute", "a; b\n*.\"label\": child", "unsupported_feature", nil},
		{"wildcard child map", "a; b\n*: {child: {leaf}}", "unsupported_feature", nil},
		{"wildcard classes root", "a; b\n*: {classes: {c: {shape: circle}}}", "unsupported_feature", nil},
		{"map variable", "vars: {template: {a; b}}\nx: ${template}", "unsupported_feature", nil},
		{"array variable", "vars: {template: [a; b]}\nx: ${template}", "unsupported_feature", nil},
		{"substitution doubling", "vars: {a: abc; b: \"${a}${a}\"; c: \"${b}${b}\"}\nx: ${c}", "unsupported_feature", nil},
		{"composite spread", "vars: {a: literal}\n...${a}", "unsupported_feature", nil},
		{"reserved variable substitution", "vars: {d2-config: {pad: 10}}\nx: ${d2-config}", "unsupported_feature", nil},
		{"structural class", "classes: {c: {child: {leaf}}}\na.class: c", "unsupported_feature", nil},
		{"recursive class", "classes: {c: {class: c}}\na.class: c", "unsupported_feature", nil},
		{"quoted structural class", "classes: {c: {\"label\": child}}\na.class: c", "unsupported_feature", nil},
		{"block substitution", "vars: {a: abc}\nx: |md ${a} ${a} |", "unsupported_feature", nil},
		{"recursive block variable", "vars: {a: abc; b: |md ${a} ${a} |}\nx: ${b}", "unsupported_feature", nil},
		{"imported nesting", "...@deep", "limit_exceeded", fstest.MapFS{"deep.d2": {Data: []byte(strings.Repeat("a: {", 18) + "z" + strings.Repeat("}", 18))}}},
		{"composed import depth", "a.b.c.d.e.f.g.h: @other", "limit_exceeded", fstest.MapFS{"other.d2": {Data: []byte("i.j.k.l.m.n.o.p.q")}}},
		{"imported wildcard edges", "...@other", "unsupported_feature", fstest.MapFS{"other.d2": {Data: []byte("a; b\n* -> *")}}},
		{"import cycle", "...@a", "unsupported_feature", fstest.MapFS{"a.d2": {Data: []byte("...@b")}, "b.d2": {Data: []byte("...@a")}}},
		{"class import", "classes: {c: @other}", "unsupported_feature", fstest.MapFS{"other.d2": {Data: []byte("a")}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := preflight(context.Background(), Source{Code: tc.code, Path: "input.d2", FS: tc.files})
			var failure *Error
			if !errors.As(err, &failure) || failure.Code != tc.want {
				t.Fatalf("got %v, want %s", err, tc.want)
			}
		})
	}
}

func TestPreflightBudgetsRepeatedWork(t *testing.T) {
	var objects strings.Builder
	for i := 0; i < 250; i++ {
		fmt.Fprintf(&objects, "n%d\n", i)
	}
	cases := []struct {
		name, code string
		files      fstest.MapFS
	}{
		{"small all-pairs input", objects.String() + "* -> *", nil},
		{"many scalar copies", "vars: {v: \"" + strings.Repeat("x", 6000) + "\"}\n" + strings.Repeat("a: ${v}\n", 50), nil},
		{"case folded scalar copies", "vars: {v: \"" + strings.Repeat("x", 6000) + "\"}\n" + strings.Repeat("a: ${V}\n", 50), nil},
		{"unicode folded scalar copies", "vars: {σ: \"" + strings.Repeat("x", 6000) + "\"}\n" + strings.Repeat("a: ${ς}\n", 50), nil},
		{"class applications", "classes: {c: {label: \"" + strings.Repeat("x", 6000) + "\"}}\n" + strings.ReplaceAll(objects.String(), "\n", ".class: c\n"), nil},
		{"style applications", objects.String() + strings.Repeat("*.style.fill: red\n", 20), nil},
		{"board inheritance", objects.String() + "scenarios: {" + strings.Repeat("x: {a}\n", 1) + "y: {b}; z: {c}; w: {d}; e: {e}; f: {f}; g: {g}; h: {h}; i: {i}; j: {j}; k: {k}; l: {l}; m: {m}}", nil},
		{"import duplication", "...@a\n...@a", fstest.MapFS{"a.d2": {Data: []byte("...@b\n...@b")}, "b.d2": {Data: []byte("...@c\n...@c")}, "c.d2": {Data: []byte("x: \"" + strings.Repeat("x", 35000) + "\"")}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := preflight(context.Background(), Source{Code: tc.code, Path: "input.d2", FS: tc.files})
			var failure *Error
			if !errors.As(err, &failure) {
				t.Fatalf("expected bounded admission error, got %v", err)
			}
		})
	}
}

func TestPreflightPreservesBoundedD2(t *testing.T) {
	cases := []struct {
		name, code string
		files      fstest.MapFS
	}{
		{"shapes", "a -> b\nb.shape: cylinder", nil},
		{"admitted depth", strings.Repeat("a: {", maxCompileDepth-1) + "z" + strings.Repeat("}", maxCompileDepth-1), nil},
		{"scalar vars", "vars: {title: Example; color: red}\na: ${title}\na.style.fill: ${color}", nil},
		{"case folded variables", "vars: {title: Example}\na: ${TITLE}", nil},
		{"quoted reserved objects", "\"vars\": {a: {b}}\n\"classes\": {a: {b}}\n\"layers\": {a: {b}}", nil},
		{"quoted literal star", "\"*\": star\n\"*\".child: child", nil},
		{"classes", "classes: {service: {shape: rectangle; style: {fill: lightblue}}}\na.class: service\nb.class: service\na -> b", nil},
		{"style globs", "a; b; c\n*.style.fill: red\n*: {style.stroke: blue}", nil},
		{"glob classes", "classes: {c: {shape: circle}}\na; b\n*.class: c", nil},
		{"indexed edge styling", "a -> b\n(* -> *)[*].style.stroke: red", nil},
		{"config and legend", "vars: {d2-config: {layout-engine: elk; pad: 20}; d2-legend: {a: Example}}\na", nil},
		{"boards", "a; scenarios: {next: {b; a -> b}}\nlayers: {detail: {c}}", nil},
		{"root import", "...@common\na -> b", fstest.MapFS{"common.d2": {Data: []byte("a; b")}}},
		{"nested imports", "container: @sub/common", fstest.MapFS{"sub/common.d2": {Data: []byte("...@leaf")}, "sub/leaf.d2": {Data: []byte("a -> b")}}},
		{"selected import", "a: @common.service", fstest.MapFS{"common.d2": {Data: []byte("service: {shape: cylinder}")}}},
		{"quoted braces", "a: \"" + strings.Repeat("{", 100) + "\"\nb: '" + strings.Repeat("{", 100) + "'", nil},
		{"block braces", "a: |latex " + strings.Repeat("{", 100) + strings.Repeat("}", 100) + " |", nil},
		{"block comment", "\"\"\"\n" + strings.Repeat("{", 100) + "\n\"\"\"\na", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := Source{Code: tc.code, Path: "input.d2", FS: tc.files}
			frozen, err := preflight(context.Background(), source)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := d2compiler.Compile(source.Path, strings.NewReader(source.Code), &d2compiler.CompileOptions{FS: frozen}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPreflightFreezesImportsAndPreservesParseDiagnostics(t *testing.T) {
	backing := fstest.MapFS{"common.d2": {Data: []byte("a")}}
	source := Source{Code: "...@common", Path: "input.d2", FS: backing}
	frozen, err := preflight(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	backing["common.d2"].Data = []byte("* -> *")
	data, err := fs.ReadFile(frozen, "common.d2")
	if err != nil || string(data) != "a" {
		t.Fatalf("snapshot = %q, %v", data, err)
	}
	if _, err := frozen.Open("unchecked.d2"); !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("unexpected unchecked read: %v", err)
	}
	_, err = preflight(context.Background(), Source{Code: "...@broken", Path: "input.d2", FS: fstest.MapFS{"broken.d2": {Data: []byte("a: {")}}})
	var parse *d2parser.ParseError
	if !errors.As(err, &parse) || len(parse.Errors) == 0 || parse.Errors[0].Range.Path != "broken.d2" {
		t.Fatalf("lost located parse diagnostics: %v", err)
	}
}
