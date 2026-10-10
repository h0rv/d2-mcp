// Package d2 contains the bounded, request-local D2 rendering pipeline.
package d2

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"math"
	"net/url"
	"strconv"
	"strings"

	d2log "github.com/d2lang/d2/lib/log"

	"github.com/d2lang/d2/d2compiler"
	"github.com/d2lang/d2/d2graph"
	"github.com/d2lang/d2/d2layouts/d2dagrelayout"
	"github.com/d2lang/d2/d2layouts/d2elklayout"
	"github.com/d2lang/d2/d2layouts/d2talalayout"
	"github.com/d2lang/d2/d2lib"
	"github.com/d2lang/d2/d2renderers/d2ascii"
	"github.com/d2lang/d2/d2renderers/d2ascii/asciishapes"
	"github.com/d2lang/d2/d2renderers/d2ascii/charset"
	"github.com/d2lang/d2/d2renderers/d2raster"
	"github.com/d2lang/d2/d2renderers/d2scenebuild"
	"github.com/d2lang/d2/d2renderers/d2svg"
	"github.com/d2lang/d2/d2target"
	"github.com/d2lang/d2/d2themes/d2themescatalog"
	"github.com/d2lang/d2/lib/textmeasure"
)

const MaxSourceBytes = 256 << 10
const MaxArtifactBytes = 16 << 20
const maxDimension = 8192
const maxPixels = 16_000_000

// Options use pointers so explicit false and zero override source configuration.
type Options struct {
	Layout      *string  `json:"layout,omitempty"`
	ThemeID     *int64   `json:"theme_id,omitempty"`
	DarkThemeID *int64   `json:"dark_theme_id,omitempty"`
	Sketch      *bool    `json:"sketch,omitempty"`
	Padding     *int64   `json:"padding,omitempty"`
	Scale       *float64 `json:"scale,omitempty"`
	Center      *bool    `json:"center,omitempty"`
}

type Error struct{ Code, Message string }

func (e *Error) Error() string           { return e.Message }
func failure(code, message string) error { return &Error{code, message} }

type Source struct {
	Code, Path string
	FS         fs.FS
}
type Prepared struct {
	Diagram *d2target.Diagram
	Options *d2svg.RenderOpts
	Layout  string
}

func LayoutResolver(engine string) (d2graph.LayoutGraph, error) {
	switch engine {
	case "dagre":
		return d2dagrelayout.DefaultLayout, nil
	case "elk":
		return d2elklayout.DefaultLayout, nil
	case "tala":
		return d2talalayout.DefaultLayout, nil
	default:
		return nil, failure("invalid_arguments", "layout must be dagre, elk, or tala")
	}
}

func checkOptions(o Options) error {
	if o.Layout != nil {
		if _, err := LayoutResolver(*o.Layout); err != nil {
			return err
		}
	}
	for _, theme := range []*int64{o.ThemeID, o.DarkThemeID} {
		if theme != nil && d2themescatalog.Find(*theme).Name == "" {
			return failure("invalid_arguments", "unknown theme ID")
		}
	}
	if o.Padding != nil && (*o.Padding < 0 || *o.Padding > 1000) {
		return failure("invalid_arguments", "padding must be between 0 and 1000")
	}
	if o.Scale != nil && (math.IsNaN(*o.Scale) || math.IsInf(*o.Scale, 0) || *o.Scale < 0.1 || *o.Scale > 4) {
		return failure("invalid_arguments", "scale must be between 0.1 and 4")
	}
	return nil
}

// Prepare validates semantics and boundaries before layout. The caller supplies
// a bounded snapshot FS; a nil FS must never reach D2's ambient filesystem fallback.
func Prepare(ctx context.Context, source Source, o Options) (*Prepared, error) {
	ctx = d2log.With(ctx, slog.Default())
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(source.Code) > MaxSourceBytes {
		return nil, failure("limit_exceeded", "source exceeds 256 KiB")
	}
	if source.FS == nil {
		return nil, errors.New("source filesystem is required")
	}
	if err := checkOptions(o); err != nil {
		return nil, err
	}
	graph, config, err := d2compiler.Compile(source.Path, strings.NewReader(source.Code), &d2compiler.CompileOptions{FS: source.FS})
	if err != nil {
		return nil, err
	}
	if err := checkGraph(graph); err != nil {
		return nil, err
	}
	// Match D2's precedence: explicit arguments, source vars.d2-config, defaults.
	if config != nil {
		if o.Layout == nil {
			o.Layout = config.LayoutEngine
		}
		if o.ThemeID == nil {
			o.ThemeID = config.ThemeID
		}
		if o.DarkThemeID == nil {
			o.DarkThemeID = config.DarkThemeID
		}
		if o.Sketch == nil {
			o.Sketch = config.Sketch
		}
		if o.Padding == nil {
			o.Padding = config.Pad
		}
		if o.Center == nil {
			o.Center = config.Center
		}
		if seeds, ok := config.Data["tala-seeds"]; ok {
			if list, ok := seeds.([]interface{}); !ok || len(list) > 16 {
				return nil, failure("limit_exceeded", "tala-seeds must be an array of at most 16 seeds")
			}
		}
	}
	if err := checkOptions(o); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ruler, err := textmeasure.NewRuler()
	if err != nil {
		return nil, err
	}
	render := &d2svg.RenderOpts{ThemeID: o.ThemeID, DarkThemeID: o.DarkThemeID, Sketch: o.Sketch, Pad: o.Padding, Scale: o.Scale, Center: o.Center}
	compile := &d2lib.CompileOptions{FS: source.FS, InputPath: source.Path, Ruler: ruler, Layout: o.Layout, LayoutResolver: LayoutResolver,
		RouterResolver: func(engine string) (d2graph.RouteEdges, error) {
			if engine == "tala" {
				return d2talalayout.RouteEdges, nil
			}
			return nil, nil
		},
	}
	diagram, _, err := d2lib.Compile(ctx, source.Code, compile, render)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	tl, br := diagram.BoundingBox()
	scale := 1.0
	if render.Scale != nil {
		scale = *render.Scale
	}
	width := (float64(br.X) - float64(tl.X) + 2*float64(*render.Pad)) * scale
	height := (float64(br.Y) - float64(tl.Y) + 2*float64(*render.Pad)) * scale
	if width > maxDimension || height > maxDimension || width*height > maxPixels || width < 0 || height < 0 {
		return nil, failure("limit_exceeded", "diagram exceeds 8192 pixels per side or 16 million pixels; reduce size, padding, or scale")
	}
	return &Prepared{diagram, render, *compile.Layout}, nil
}

func checkGraph(g *d2graph.Graph) error {
	if len(g.Layers)+len(g.Scenarios)+len(g.Steps) > 0 {
		return failure("unsupported_feature", "multi-board diagrams are not supported; submit one board per call")
	}
	objects, edges := g.Objects, g.Edges
	if g.Legend != nil {
		objects = append(append([]*d2graph.Object{}, objects...), g.Legend.Objects...)
		edges = append(append([]*d2graph.Edge{}, edges...), g.Legend.Edges...)
	}
	if len(objects) > 1000 || len(edges) > 2000 {
		return failure("limit_exceeded", "diagram exceeds 1000 objects or 2000 connections")
	}
	check := func(a d2graph.Attributes) error {
		if a.Icon != nil {
			return failure("unsupported_feature", "icons and external image assets are disabled; use native D2 shapes")
		}
		if a.Link != nil && !SafeLink(a.Link.Value) {
			return failure("unsupported_feature", "links must use http, https, mailto, or a fragment")
		}
		for _, dim := range []*d2graph.Scalar{a.WidthAttr, a.HeightAttr, a.Top, a.Left} {
			if dim != nil {
				v, err := strconv.ParseFloat(dim.Value, 64)
				if err != nil || math.Abs(v) > maxDimension {
					return failure("limit_exceeded", "explicit dimensions and positions must be within 8192")
				}
			}
		}
		return nil
	}
	if err := check(g.Root.Attributes); err != nil {
		return err
	}
	for _, obj := range objects {
		if err := check(obj.Attributes); err != nil {
			return err
		}
	}
	for _, edge := range edges {
		if err := check(edge.Attributes); err != nil {
			return err
		}
	}
	return nil
}

func SafeLink(value string) bool {
	if strings.HasPrefix(value, "#") {
		return true
	}
	u, err := url.Parse(value)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http" || u.Scheme == "mailto")
}

func Render(ctx context.Context, p *Prepared, format, asciiMode string) ([]byte, error) {
	ctx = d2log.With(ctx, slog.Default())
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if format != "svg" && p.Options.DarkThemeID != nil {
		return nil, failure("unsupported_feature", "dark_theme_id is only supported for SVG output")
	}
	var data []byte
	var err error
	switch format {
	case "svg":
		data, err = d2svg.Render(p.Diagram, p.Options)
	case "ascii":
		if err := checkASCIICanvas(p.Diagram); err != nil {
			return nil, err
		}
		opts := &d2ascii.RenderOpts{Charset: charset.Unicode}
		if asciiMode == "standard" {
			opts.Charset = charset.ASCII
		}
		data, err = d2ascii.NewASCIIartist().Render(ctx, p.Diagram, opts)
	case "png":
		opts := p.Options
		doc, buildErr := d2scenebuild.Build(ctx, p.Diagram, d2scenebuild.Options{
			Pad: opts.Pad, Scale: opts.Scale, Center: opts.Center, ThemeID: opts.ThemeID, ThemeOverrides: opts.ThemeOverrides, Sketch: *opts.Sketch,
			MaxNodes: 100_000, MaxPathCommands: 1_000_000,
			SketchBudget: d2scenebuild.SketchBudget{MaxOperationSets: 100_000, MaxOperations: 1_000_000, MaxPathCommands: 1_000_000},
			LinkBudget:   d2scenebuild.LinkBudget{MaxRegions: 10_000, MaxStringBytes: MaxSourceBytes},
		})
		if buildErr != nil {
			return nil, buildErr
		}
		frame, renderErr := d2raster.Render(ctx, doc, d2raster.FrameOptions{
			Scale: 1, MaxWidth: maxDimension, MaxHeight: maxDimension, MaxPixels: maxPixels,
			MaxNodes: 100_000, MaxDepth: 100, MaxPathCommands: 1_000_000,
			MaxAnimationTracks: 10_000, MaxAnimationKeyframes: 100_000,
			MaxAssets: 100, MaxAssetBytes: 32 << 20, MaxDecodedAssetBytes: 64 << 20, MaxImportDepth: 16,
			MaxOffscreenBytes: 64 << 20, MaxEvenOddClipWork: 100_000_000, MaxScanlineWork: 100_000_000,
		})
		if renderErr != nil {
			return nil, renderErr
		}
		data, err = d2raster.EncodePNG(ctx, frame)
	default:
		return nil, failure("invalid_arguments", fmt.Sprintf("unsupported format: %s", format))
	}
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if len(data) > MaxArtifactBytes {
		return nil, failure("limit_exceeded", "artifact exceeds 16 MiB")
	}
	return data, nil
}

// D2 0.9.0's ASCII renderer pads BOTH canvas axes by the full shape-label byte
// length. Bound its allocation before Render; image scale does not shrink ASCII.
// Route-point envelopes conservatively include upstream connection-label bounds.
func checkASCIICanvas(diagram *d2target.Diagram) error {
	if diagram.Legend != nil {
		return failure("unsupported_feature", "legends are not supported in ASCII; use SVG or PNG")
	}
	artist := d2ascii.NewASCIIartist()
	tl, br := diagram.NestedBoundingBox()
	minX, minY, maxX, maxY := float64(tl.X), float64(tl.Y), float64(br.X), float64(br.Y)
	longest := 0
	for _, shape := range diagram.Shapes {
		longest = max(longest, len(shape.Label))
	}
	for _, edge := range diagram.Connections {
		length := len(edge.Label)
		if edge.SrcLabel != nil {
			length = max(length, len(edge.SrcLabel.Label))
		}
		if edge.DstLabel != nil {
			length = max(length, len(edge.DstLabel.Label))
		}
		for _, point := range edge.Route {
			minX = math.Min(minX, point.X-float64(length)*artist.FW-1)
			maxX = math.Max(maxX, point.X+float64(length)*artist.FW+1)
			minY = math.Min(minY, point.Y-artist.FH-1)
			maxY = math.Max(maxY, point.Y+artist.FH+1)
		}
	}
	pad := float64(longest + asciishapes.MinLabelPadding + 1)
	width := math.Ceil((maxX-minX)/artist.FW*artist.SCALE) + pad
	height := math.Ceil((maxY-minY)/artist.FH*artist.SCALE) + pad
	if math.IsNaN(width) || math.IsNaN(height) || width < 0 || height < 0 || width > 2000 || height > 2000 || width*height > 1_000_000 {
		return failure("limit_exceeded", "ASCII canvas exceeds 2000 cells per side or 1 million cells; shorten labels or use SVG/PNG")
	}
	return nil
}
