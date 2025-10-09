package d2

import (
	"context"
	"log"
	"strings"

	"oss.terrastruct.com/d2/d2graph"
	"oss.terrastruct.com/d2/d2layouts/d2dagrelayout"
	"oss.terrastruct.com/d2/d2lib"
	"oss.terrastruct.com/d2/d2renderers/d2ascii"
	"oss.terrastruct.com/d2/d2renderers/d2ascii/charset"
	"oss.terrastruct.com/d2/d2renderers/d2svg"
	"oss.terrastruct.com/d2/d2target"
	"oss.terrastruct.com/d2/d2themes/d2themescatalog"
	"oss.terrastruct.com/d2/lib/textmeasure"
	"oss.terrastruct.com/util-go/go2"
)

var (
	renderOpts = &d2svg.RenderOpts{
		Sketch:  go2.Pointer(true),
		Pad:     go2.Pointer(int64(5)),
		ThemeID: &d2themescatalog.GrapeSoda.ID,
	}
	layoutResolver = func(engine string) (d2graph.LayoutGraph, error) {
		return d2dagrelayout.DefaultLayout, nil
	}
)

func Compile(ctx context.Context, code string) (diagram *d2target.Diagram, graph *d2graph.Graph, compileError error, otherError error) {
	ruler, otherErr := textmeasure.NewRuler()
	if otherErr != nil {
		log.Printf("error creating ruler: %v", otherErr)
		return nil, nil, otherErr, nil
	}

	compileOpts := &d2lib.CompileOptions{
		LayoutResolver: layoutResolver,
		Ruler:          ruler,
	}

	diagram, graph, compileErr := d2lib.Compile(ctx, code, compileOpts, renderOpts)
	return diagram, graph, compileErr, otherErr
}

func Render(ctx context.Context, code string) ([]byte, error) {
	diagram, _, compileErr, otherErr := Compile(ctx, code)
	if otherErr != nil {
		log.Printf("error compiling d2: %v", otherErr)
		return nil, otherErr
	}

	if compileErr != nil {
		log.Printf("error compiling d2: %v", compileErr)
		return nil, compileErr
	}

	out, err := d2svg.Render(diagram, renderOpts)
	if err != nil {
		log.Printf("error rendering d2: %v", err)
		return nil, err
	}

	return out, nil
}

func RenderASCII(ctx context.Context, code string, mode string) ([]byte, error) {
	diagram, _, compileErr, otherErr := Compile(ctx, code)
	if otherErr != nil {
		log.Printf("error compiling d2: %v", otherErr)
		return nil, otherErr
	}

	if compileErr != nil {
		log.Printf("error compiling d2: %v", compileErr)
		return nil, compileErr
	}

	artist := d2ascii.NewASCIIartist()
	opts := &d2ascii.RenderOpts{}

	switch strings.ToLower(mode) {
	case "standard", "ascii":
		opts.Charset = charset.ASCII
	default:
		opts.Charset = charset.Unicode
	}

	out, err := artist.Render(ctx, diagram, opts)
	if err != nil {
		log.Printf("error rendering d2 ascii: %v", err)
		return nil, err
	}

	return out, nil
}
