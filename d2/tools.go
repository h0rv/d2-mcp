package d2

import (
	"context"
	"log"

	"oss.terrastruct.com/d2/d2graph"
	"oss.terrastruct.com/d2/d2layouts/d2dagrelayout"
	"oss.terrastruct.com/d2/d2lib"
	"oss.terrastruct.com/d2/d2renderers/d2svg"
	"oss.terrastruct.com/d2/d2target"
	"oss.terrastruct.com/d2/d2themes/d2themescatalog"
	"oss.terrastruct.com/d2/lib/textmeasure"
	"oss.terrastruct.com/util-go/go2"
)

func Compile(ctx context.Context, code string) (*d2target.Diagram, *d2graph.Graph, error) {
	ruler, err := textmeasure.NewRuler()
	if err != nil {
		log.Printf("error creating ruler: %v", err)
		return nil, nil, err
	}

	layoutResolver := func(engine string) (d2graph.LayoutGraph, error) {
		return d2dagrelayout.DefaultLayout, nil
	}

	renderOpts := &d2svg.RenderOpts{
		Pad:     go2.Pointer(int64(5)),
		ThemeID: &d2themescatalog.GrapeSoda.ID,
	}

	compileOpts := &d2lib.CompileOptions{
		LayoutResolver: layoutResolver,
		Ruler:          ruler,
	}

	diagram, graph, err := d2lib.Compile(ctx, code, compileOpts, renderOpts)
	if err != nil {
		log.Printf("error compiling d2: %v", err)
		return nil, nil, err
	}

	return diagram, graph, nil
}

func Render(ctx context.Context, code string) ([]byte, error) {
	diagram, _, err := Compile(ctx, code)
	if err != nil {
		log.Printf("error compiling d2: %v", err)
		return nil, err
	}

	renderOpts := &d2svg.RenderOpts{
		Pad:     go2.Pointer(int64(5)),
		ThemeID: &d2themescatalog.GrapeSoda.ID,
	}

	out, err := d2svg.Render(diagram, renderOpts)
	if err != nil {
		log.Printf("error rendering d2: %v", err)
		return nil, err
	}

	return out, nil
}
