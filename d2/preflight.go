package d2

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/d2lang/d2/d2ast"
	"github.com/d2lang/d2/d2parser"
)

// These are admission estimates, not a CPU or memory isolation boundary. D2's
// compiler cannot be interrupted by context cancellation. Keep its structural
// input small before either of Prepare's compiler passes can run.
const maxCompileDepth = 16
const maxCompileWork = 10000

type compileAdmission struct {
	ctx                                                context.Context
	source                                             fs.FS
	files                                              map[string][]byte
	parsed                                             map[string]*d2ast.Map
	active                                             map[string]bool
	boards                                             map[string]bool
	bytes, expandedBytes, nodes, scalarBytes, targets  int
	classNodes, classBytes, classRefs, globClassRefs   int
	globNodes, globBytes, edgeGlobNodes, edgeGlobBytes int
	vars                                               map[string]int
	uses                                               []scalarUse
}
type scalarUse struct {
	name string
	glob bool
	node d2ast.Node
}

// preflight freezes exactly the files it admits. The compiler cannot open an
// unchecked file, and changing a backing FS between passes cannot bypass checks.
func preflight(ctx context.Context, source Source) (fs.FS, error) {
	if !fs.ValidPath(source.Path) {
		return nil, failure("unsupported_feature", "source path must be relative to its input root")
	}
	p := &compileAdmission{ctx: ctx, source: source.FS, files: map[string][]byte{source.Path: []byte(source.Code)}, parsed: map[string]*d2ast.Map{}, active: map[string]bool{}, boards: map[string]bool{}, vars: map[string]int{}, bytes: len(source.Code)}
	if err := p.file(source.Path, nil, 0, 0); err != nil {
		return nil, err
	}
	targets := int64(max(1, p.targets))
	// Attribute-only templates cannot create objects. Bound their applications,
	// including matching existing edge pairs, then allow for board inheritance.
	copies := int64(1 + len(p.boards))
	applications := int64(p.classRefs) + int64(p.globClassRefs)*targets
	// All factors are capped by source/AST admission above; int64 keeps these
	// conservative projections safe on 32-bit clients as well.
	work := int64(p.nodes) + int64(p.classNodes)*applications + int64(p.globNodes)*targets + int64(p.edgeGlobNodes)*targets*targets
	size := int64(p.scalarBytes) + int64(p.classBytes)*applications + int64(p.globBytes)*targets + int64(p.edgeGlobBytes)*targets*targets
	for _, use := range p.uses {
		if use.name == "d2-config" || use.name == "d2-legend" {
			return nil, admissionError(use.node, "unsupported_feature", "composite variable substitution is unsupported; use literal scalar variables or explicit source")
		}
		count := int64(1)
		if use.glob {
			count = targets
		}
		size += int64(p.vars[use.name]) * count
	}
	if work*copies > maxCompileWork || size*copies > MaxSourceBytes {
		return nil, failure("limit_exceeded", "projected compilation exceeds 10000 AST operations or 256 KiB of expanded scalar data; reduce imports, classes, style rules, variables, or boards")
	}
	return admittedFS(p.files), nil
}

func admissionError(n d2ast.Node, code, message string) error {
	r := n.GetRange()
	return failure(code, fmt.Sprintf("%s:%d:%d: %s", r.Path, r.Start.Line+1, r.Start.Column+1, message))
}
func (p *compileAdmission) file(name string, prefix []string, depth, imports int) error {
	if err := p.ctx.Err(); err != nil {
		return err
	}
	if imports > maxCompileDepth {
		return failure("limit_exceeded", "import chain exceeds 16 levels")
	}
	if p.active[name] {
		return failure("unsupported_feature", "cyclic imports are unsupported: "+name)
	}
	code, ok := p.files[name]
	if !ok {
		if !fs.ValidPath(name) || path.Ext(name) != ".d2" {
			return failure("unsupported_feature", "imports must be relative .d2 files inside the input root")
		}
		if len(p.files) >= 32 {
			return failure("limit_exceeded", "source exceeds 32 files")
		}
		f, err := p.source.Open(name)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		code, err = io.ReadAll(io.LimitReader(f, int64(MaxSourceBytes-p.bytes)+1))
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		if len(code) > MaxSourceBytes-p.bytes {
			return failure("limit_exceeded", "combined source exceeds 256 KiB")
		}
		p.files[name] = code
		p.bytes += len(code)
	}
	p.expandedBytes += len(code)
	if p.expandedBytes > MaxSourceBytes {
		return failure("limit_exceeded", "import reuse expands source beyond 256 KiB; reduce repeated imports")
	}
	ast := p.parsed[name]
	if ast == nil {
		if err := lexicalDepth(name, string(code)); err != nil {
			return err
		}
		var err error
		ast, err = d2parser.Parse(name, bytes.NewReader(code), nil)
		if err != nil {
			return err
		}
		p.parsed[name] = ast
	}
	p.active[name] = true
	defer delete(p.active, name)
	return p.walk(ast, name, prefix, depth, imports, false)
}
func indexOf(parts []string, name string) int {
	for i, s := range parts {
		if strings.EqualFold(s, name) {
			return i
		}
	}
	return -1
}
func keyParts(k *d2ast.KeyPath) []string {
	if k == nil {
		return nil
	}
	parts := k.StringIDA()
	for i, segment := range k.Path {
		// Quoted reserved-looking identifiers are ordinary object names. Retain
		// that distinction for namespace and attribute allow-list decisions.
		if !segment.Unbox().IsUnquoted() {
			parts[i] = "\x00" + parts[i]
		}
	}
	return parts
}

// Match D2's case-insensitive variable lookup, including Unicode simple folds.
func foldIdentifier(s string) string {
	return strings.Map(func(r rune) rune {
		smallest := r
		for next := unicode.SimpleFold(r); next != r; next = unicode.SimpleFold(next) {
			smallest = min(smallest, next)
		}
		return unicode.ToLower(smallest)
	}, s)
}
func concat(a, b []string) []string {
	r := make([]string, 0, len(a)+len(b))
	r = append(r, a...)
	return append(r, b...)
}

func (p *compileAdmission) walk(n d2ast.Node, file string, prefix []string, depth, imports int, inGlob bool) error {
	if err := p.ctx.Err(); err != nil {
		return err
	}
	p.nodes++
	if p.nodes > maxCompileWork {
		return admissionError(n, "limit_exceeded", "expanded AST exceeds 10000 operations")
	}
	inClass := indexOf(prefix, "classes") >= 0
	if inClass {
		p.classNodes++
	}
	if scalar, ok := n.(d2ast.Scalar); ok {
		size := len(scalar.ScalarString())
		p.scalarBytes += size
		if inClass {
			p.classBytes += size
		}
		if p.scalarBytes > MaxSourceBytes {
			return admissionError(n, "limit_exceeded", "expanded scalar data exceeds 256 KiB")
		}
	}
	switch n := n.(type) {
	case *d2ast.Import:
		if len(n.Path) > maxCompileDepth {
			return admissionError(n, "limit_exceeded", "import selection exceeds 16 key segments")
		}
		if inClass || indexOf(prefix, "vars") >= 0 || inGlob {
			return admissionError(n, "unsupported_feature", "imports inside variable/class definitions or style rules are unsupported; import explicit source at a diagram scope")
		}
		name := n.PathWithPre()
		if strings.HasPrefix(name, "/") {
			return admissionError(n, "unsupported_feature", "imports must stay inside the input root")
		}
		if path.Ext(name) != ".d2" {
			name += ".d2"
		}
		name = path.Join(path.Dir(file), name)
		// The upstream compiler compiles the entire library even for @file.selection.
		return p.file(name, prefix, depth, imports+1)
	case *d2ast.Substitution:
		if n.Spread || inClass || indexOf(prefix, "vars") >= 0 || len(n.Path) != 1 {
			return admissionError(n, "unsupported_feature", "structural or recursively defined substitutions are unsupported; use literal scalar variables and explicit diagram source")
		}
		p.uses = append(p.uses, scalarUse{foldIdentifier(n.Path[0].ScalarString()), inGlob, n})
	case *d2ast.BlockString:
		// Goldmark performs these substitutions outside the D2 AST, including
		// repeated string replacement. Do not admit that second expansion language.
		if strings.Contains(n.Value, "${") {
			return admissionError(n, "unsupported_feature", "block-string interpolation is unsupported; use an ordinary quoted label with literal scalar variables")
		}
	case *d2ast.Key:
		parts := keyParts(n.Key)
		full := concat(prefix, parts)
		nextDepth := depth + len(parts)
		edgeDepth := 0
		for _, e := range n.Edges {
			edgeDepth = max(edgeDepth, len(e.Src.Path), len(e.Dst.Path))
		}
		nextDepth += edgeDepth + len(keyParts(n.EdgeKey))
		if nextDepth > maxCompileDepth {
			return admissionError(n, "limit_exceeded", "combined map and dotted-key nesting exceeds 16 levels; flatten the diagram")
		}
		p.targets += max(1, len(parts)) + len(n.Edges)*2
		if v := indexOf(full, "vars"); v >= 0 {
			tail := full[v+1:]
			if len(tail) > 0 && !strings.EqualFold(tail[0], "d2-config") && !strings.EqualFold(tail[0], "d2-legend") {
				literal, ok := n.Value.Unbox().(d2ast.Scalar)
				if len(tail) != 1 || !ok || len(literal.Children()) != 0 {
					return admissionError(n, "unsupported_feature", "variable definitions must be literal scalars; map, array, and recursively substituted variables are unsupported")
				}
				name := foldIdentifier(strings.TrimPrefix(tail[0], "\x00"))
				p.vars[name] = max(p.vars[name], len(literal.ScalarString()))
			}
		}
		if c := indexOf(full, "classes"); c >= 0 {
			tail := full[c+1:]
			if len(tail) == 1 && n.Value.Map != nil {
				if !attributeTree(n.Value.Map, nil, false) {
					return admissionError(n, "unsupported_feature", "classes may contain only literal attributes; structural templates and nested class references are unsupported")
				}
			} else if len(tail) > 1 && !attributePath(tail[1:], false) {
				return admissionError(n, "unsupported_feature", "classes may contain only literal attributes; expand structural templates explicitly")
			}
		}
		for i, s := range full {
			if (strings.EqualFold(s, "layers") || strings.EqualFold(s, "scenarios") || strings.EqualFold(s, "steps")) && i+1 < len(full) {
				p.boards[strings.Join(full[:i+2], "\x00")] = true
			}
		}
		if len(p.boards) >= 32 {
			return admissionError(n, "limit_exceeded", "source exceeds 32 boards including the root")
		}
		glob := n.Key.HasGlob() || n.EdgeKey.HasGlob() || (n.EdgeIndex != nil && n.EdgeIndex.Glob)
		for _, e := range n.Edges {
			glob = glob || e.Src.HasGlob() || e.Dst.HasGlob()
		}
		if glob {
			if len(n.Edges) > 0 {
				if n.EdgeIndex == nil {
					return admissionError(n, "unsupported_feature", "wildcard edge creation is unsupported; list edges explicitly or use indexed existing-edge styling")
				}
				if !attributeTree(n.Value.Unbox(), keyParts(n.EdgeKey), true) {
					return admissionError(n, "unsupported_feature", "edge wildcard rules may only set attributes of indexed existing edges")
				}
			} else {
				first := n.Key.FirstGlob()
				if !attributeTree(n.Value.Unbox(), parts[first+1:], true) {
					return admissionError(n, "unsupported_feature", "object wildcard rules may only set attributes on existing objects; list child objects explicitly")
				}
			}
			nodes, size := subtreeCost(n)
			if len(n.Edges) > 0 {
				p.edgeGlobNodes += nodes
				p.edgeGlobBytes += size
			} else {
				p.globNodes += nodes
				p.globBytes += size
			}
		}
		if len(parts) > 0 && strings.EqualFold(parts[len(parts)-1], "class") && !inClass {
			refs := 1
			if n.Value.Array != nil {
				refs = len(n.Value.Array.Nodes)
			}
			if inGlob || glob {
				p.globClassRefs += refs
			} else {
				p.classRefs += refs
			}
		}
		for _, child := range n.Children() {
			if err := p.walk(child, file, full, nextDepth, imports, inGlob || glob); err != nil {
				return err
			}
		}
		return nil
	case *d2ast.Array:
		depth++
		if depth > maxCompileDepth {
			return admissionError(n, "limit_exceeded", "array nesting exceeds 16 levels")
		}
	}
	for _, child := range n.Children() {
		if err := p.walk(child, file, prefix, depth, imports, inGlob); err != nil {
			return err
		}
	}
	return nil
}

// This allow-list intentionally excludes reserved structural roots (vars,
// classes, layers, scenarios, steps). ReservedKeywords alone is not safe here.
func attributePath(parts []string, allowClass bool) bool {
	if len(parts) == 0 {
		return true
	}
	head := strings.ToLower(parts[0])
	switch head {
	case "style":
		if len(parts) == 1 {
			return true
		}
		_, ok := d2ast.StyleKeywords[strings.ToLower(parts[1])]
		return ok && len(parts) == 2
	case "source-arrowhead", "target-arrowhead":
		return attributePath(parts[1:], false)
	case "label", "icon", "tooltip":
		return len(parts) == 1 || (len(parts) == 2 && (strings.EqualFold(parts[1], "near") || strings.EqualFold(parts[1], "style"))) || (len(parts) > 2 && strings.EqualFold(parts[1], "style") && attributePath(parts[1:], false))
	case "class":
		return allowClass && len(parts) == 1
	case "vars":
		return false
	default:
		_, ok := d2ast.SimpleReservedKeywords[head]
		return ok && len(parts) == 1
	}
}
func attributeTree(n d2ast.Node, prefix []string, allowClass bool) bool {
	if !attributePath(prefix, allowClass) {
		return false
	}
	if n == nil {
		return len(prefix) > 0
	}
	switch n := n.(type) {
	case *d2ast.Map:
		for _, box := range n.Nodes {
			k := box.MapKey
			if k == nil {
				if box.Comment != nil || box.BlockComment != nil {
					continue
				}
				return false
			}
			if len(k.Edges) > 0 || k.Key.HasGlob() {
				return false
			}
			if !attributeTree(k.Value.Unbox(), concat(prefix, keyParts(k.Key)), allowClass) {
				return false
			}
			if !allowClass && k.Primary.Unbox() != nil && len(k.Primary.Unbox().Children()) != 0 {
				return false
			}
		}
		return true
	case *d2ast.Array:
		if len(prefix) == 0 {
			return false
		}
		for _, box := range n.Nodes {
			if !attributeTree(box.Unbox(), prefix, allowClass) {
				return false
			}
		}
		return true
	case *d2ast.Import:
		return false
	case *d2ast.Substitution:
		return allowClass && !n.Spread && len(n.Path) == 1 && len(prefix) > 0
	case d2ast.Scalar:
		if len(prefix) == 0 {
			return false
		}
		for _, child := range n.Children() {
			if !attributeTree(child, prefix, allowClass) {
				return false
			}
		}
		return true
	case *d2ast.Comment, *d2ast.BlockComment:
		return true
	default:
		return false
	}
}
func subtreeCost(n d2ast.Node) (int, int) {
	nodes, size := 0, 0
	stack := []d2ast.Node{n}
	for len(stack) > 0 {
		last := len(stack) - 1
		n = stack[last]
		stack = stack[:last]
		nodes++
		if scalar, ok := n.(d2ast.Scalar); ok {
			size += len(scalar.ScalarString())
		}
		stack = append(stack, n.Children()...)
	}
	return nodes, size
}

// lexicalDepth runs before the recursive parser. It recognizes D2's string and
// comment delimiters rather than counting braces in Markdown, LaTeX or quotes.
// The AST walk subsequently applies the tighter cumulative dotted-key bound.
func lexicalDepth(name, code string) error {
	if !utf8.ValidString(code) || strings.IndexByte(code, 0) >= 0 {
		return failure("unsupported_feature", name+": source must be UTF-8 text without NUL bytes")
	}
	type state struct{ value, array, token, entry bool }
	s := state{token: true, entry: true}
	stack := []state{}
	for i := 0; i < len(code); {
		c := code[i]
		if c == '\\' {
			i++
			if i < len(code) {
				_, size := utf8.DecodeRuneInString(code[i:])
				i += size
			}
			s.token = false
			s.entry = false
			continue
		}
		if c == '#' {
			end := strings.IndexByte(code[i:], '\n')
			if end < 0 {
				return nil
			}
			i += end
			continue
		}
		if c == '\n' || c == ';' {
			s.value = s.array
			s.token = true
			s.entry = true
			i++
			continue
		}
		r, runeSize := utf8.DecodeRuneInString(code[i:])
		if unicode.IsSpace(r) {
			i += runeSize
			continue
		}
		if s.token && (c == '"' || c == '\'') {
			if strings.HasPrefix(code[i:], `"""`) && s.entry {
				end := strings.Index(code[i+3:], `"""`)
				if end < 0 {
					return nil
				}
				i += end + 6
				continue
			}
			quote := c
			i++
			for i < len(code) {
				if quote == '\'' && code[i] == '\\' && i+1 < len(code) && code[i+1] == '\n' {
					i += 2
					continue
				}
				if code[i] == '\n' {
					break
				}
				if quote == '"' && code[i] == '\\' {
					i = min(i+2, len(code))
					continue
				}
				if code[i] == quote {
					i++
					if quote == '\'' && i < len(code) && code[i] == '\'' {
						i++
						continue
					}
					break
				}
				i++
			}
			s.token = false
			s.entry = false
			continue
		}
		if s.token && c == '|' {
			i++
			start := i
			for i < len(code) {
				r, size := utf8.DecodeRuneInString(code[i:])
				if unicode.IsSpace(r) || unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
					break
				}
				i += size
			}
			quote := code[start:i]
			for i < len(code) {
				r, size := utf8.DecodeRuneInString(code[i:])
				if unicode.IsSpace(r) {
					break
				}
				i += size
			}
			ending := "|"
			if quote != "" {
				hint, size := utf8.DecodeLastRuneInString(quote)
				ending = string(hint) + quote[size:] + "|"
			}
			end := strings.Index(code[i:], ending)
			if end < 0 {
				return nil
			}
			i += end + len(ending)
			s.token = false
			s.entry = false
			continue
		}
		switch c {
		case '{', '[':
			stack = append(stack, s)
			if len(stack) > maxCompileDepth {
				return failure("limit_exceeded", name+": map/array nesting exceeds 16 levels; flatten the diagram")
			}
			s = state{value: c == '[', array: c == '[', token: true, entry: true}
		case '}', ']':
			if len(stack) > 0 {
				s = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
			}
			s.token = false
			s.entry = false
		case ':':
			if !s.value {
				s.value = true
				s.token = true
				s.entry = false
			} else {
				s.token = false
			}
		case '.', '<', '>':
			s.token = !s.value
			s.entry = false
		case '(', '&':
			// These are selectors only at a key boundary; embedded parentheses
			// in an unquoted key are ordinary text to the upstream parser.
			s.token = s.token && !s.value
			s.entry = false
		case ')':
			s.token = false
			s.entry = false
		case '@':
			// Imports introduce a string only at a token boundary. An @ inside
			// an unquoted label, or a bare @ at a map-key boundary, must not
			// hide later braces. Root imports require the ...@ spread form.
			s.token = s.token && (s.value || strings.HasSuffix(code[:i], "..."))
			s.entry = false
		case '-':
			if !s.value && i+1 < len(code) && (code[i+1] == '>' || code[i+1] == '-' || code[i+1] == '*') {
				i++
				s.token = true
			} else {
				s.token = false
			}
			s.entry = false
		default:
			s.token = false
			s.entry = false
		}
		i++
	}
	return nil
}

type admittedFS map[string][]byte

func (a admittedFS) Open(name string) (fs.File, error) {
	data, ok := a[name]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
	}
	return &admittedFile{Reader: bytes.NewReader(data), name: name, size: int64(len(data))}, nil
}

type admittedFile struct {
	*bytes.Reader
	name string
	size int64
}

func (f *admittedFile) Close() error               { return nil }
func (f *admittedFile) Stat() (fs.FileInfo, error) { return admittedInfo{f.name, f.size}, nil }

type admittedInfo struct {
	name string
	size int64
}

func (i admittedInfo) Name() string       { return path.Base(i.name) }
func (i admittedInfo) Size() int64        { return i.size }
func (i admittedInfo) Mode() fs.FileMode  { return 0444 }
func (i admittedInfo) ModTime() time.Time { return time.Time{} }
func (i admittedInfo) IsDir() bool        { return false }
func (i admittedInfo) Sys() any           { return nil }
