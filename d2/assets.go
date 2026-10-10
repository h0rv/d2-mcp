package d2

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/d2lang/d2/d2renderers/d2raster"
	"github.com/d2lang/d2/d2renderers/d2scene"
	"github.com/d2lang/d2/d2renderers/d2scenebuild"
	"github.com/d2lang/d2/d2renderers/d2svgimport"
	"github.com/d2lang/d2/d2target"
	"github.com/d2lang/d2/lib/imageasset"
)

const MaxAssetBytes = 1 << 20
const MaxAssetDimension = 1024

// Assets are admitted once per request and normalized to PNG. SVG's strict,
// network-free importer is used before rasterization; original SVG markup never
// reaches the returned artifact. Even enabled assets cannot trigger URL fetching.
type assetLoader struct {
	ctx           context.Context
	fs            fs.FS
	inline        bool
	cache         map[string]*url.URL
	bytes, pixels int
}

func newAssetLoader(ctx context.Context, files fs.FS, inline bool) *assetLoader {
	return &assetLoader{ctx: ctx, fs: files, inline: inline, cache: make(map[string]*url.URL)}
}

func (a *assetLoader) load(source *url.URL) (*url.URL, error) {
	if err := a.ctx.Err(); err != nil {
		return nil, err
	}
	key := source.String()
	if cached := a.cache[key]; cached != nil {
		return cached, nil
	}
	if len(a.cache) >= 32 {
		return nil, failure("limit_exceeded", "source exceeds 32 distinct image assets")
	}
	var data []byte
	var err error
	var svg bool
	switch {
	case source.Scheme == "data" && a.inline:
		if len(key) > base64.StdEncoding.EncodedLen(MaxAssetBytes)+128 {
			return nil, failure("limit_exceeded", "image asset exceeds 1 MiB")
		}
		prefix, encoded, ok := strings.Cut(source.Opaque, ",")
		if !ok {
			return nil, failure("unsupported_feature", "asset data URL must be base64 PNG, JPEG, GIF, or SVG")
		}
		switch prefix {
		case "image/png;base64", "image/jpeg;base64", "image/gif;base64":
		case "image/svg+xml;base64":
			svg = true
		default:
			return nil, failure("unsupported_feature", "asset data URL must be base64 PNG, JPEG, GIF, or SVG")
		}
		data, err = base64.StdEncoding.DecodeString(encoded)
	case source.Scheme == "" && source.Host == "" && source.RawQuery == "" && source.Fragment == "" && a.fs != nil:
		name := source.Path
		if !fs.ValidPath(name) || strings.Contains(name, "\\") {
			return nil, failure("unsupported_feature", "asset paths must be relative to --asset-dir")
		}
		switch strings.ToLower(path.Ext(name)) {
		case ".png", ".jpg", ".jpeg", ".gif":
		case ".svg":
			svg = true
		default:
			return nil, failure("unsupported_feature", "asset files must be PNG, JPEG, GIF, or SVG")
		}
		info, statErr := fs.Stat(a.fs, name)
		if statErr != nil || !info.Mode().IsRegular() {
			return nil, failure("unsupported_feature", "asset must be a regular file inside --asset-dir")
		}
		f, openErr := a.fs.Open(name)
		if openErr != nil {
			return nil, failure("unsupported_feature", "asset is unavailable inside --asset-dir")
		}
		defer f.Close()
		info, statErr = f.Stat()
		if statErr != nil || !info.Mode().IsRegular() {
			return nil, failure("unsupported_feature", "asset must be a regular file")
		}
		data, err = io.ReadAll(io.LimitReader(f, MaxAssetBytes+1))
	default:
		return nil, failure("unsupported_feature", "image assets require --asset-dir or --allow-inline-assets; network and absolute paths are disabled")
	}
	if err := a.ctx.Err(); err != nil {
		return nil, err
	}
	if err != nil {
		return nil, failure("unsupported_feature", "cannot decode or read image asset")
	}
	if len(data) > MaxAssetBytes || a.bytes+len(data) > 8<<20 {
		return nil, failure("limit_exceeded", "image assets exceed 1 MiB each or 8 MiB combined")
	}
	a.bytes += len(data)
	var raster image.Image
	if svg {
		imported, importErr := d2svgimport.ImportNode(a.ctx, "asset.svg", data, d2svgimport.Limits{
			MaxBytes: MaxAssetBytes, MaxDepth: 32, MaxElements: 5000, MaxAttributes: 20000, MaxAttributeBytes: MaxAssetBytes,
			MaxPathCommands: 50000, MaxTransformFunctions: 5000, MaxUseDepth: 16, MaxResources: 256,
		})
		if err := a.ctx.Err(); err != nil {
			return nil, err
		}
		if importErr != nil {
			return nil, failure("unsupported_feature", fmt.Sprintf("unsupported SVG asset: %v", importErr))
		}
		if imported.Width <= 0 || imported.Height <= 0 || imported.Width > MaxAssetDimension || imported.Height > MaxAssetDimension {
			return nil, failure("limit_exceeded", "image assets must be at most 1024 pixels per side")
		}
		a.pixels += int(imported.Width+1) * int(imported.Height+1)
		if a.pixels > 8<<20 {
			return nil, failure("limit_exceeded", "image assets exceed 8 million decoded pixels")
		}
		root := d2scene.NewNode(nil)
		root.Transform = imported.ViewportTransform
		root.Children = []*d2scene.Node{imported.Root}
		doc := d2scene.NewDocument(d2scene.Box{Width: imported.Width, Height: imported.Height}, root)
		doc.Assets = imported.Assets
		frame, renderErr := d2raster.Render(a.ctx, doc, d2raster.FrameOptions{
			Scale: 1, MaxWidth: MaxAssetDimension, MaxHeight: MaxAssetDimension, MaxPixels: 1 << 20,
			MaxNodes: 10000, MaxDepth: 64, MaxPathCommands: 50000, MaxAnimationTracks: 1, MaxAnimationKeyframes: 1,
			MaxAssets: 256, MaxAssetBytes: MaxAssetBytes, MaxDecodedAssetBytes: 4 << 20, MaxImportDepth: 16,
			MaxOffscreenBytes: 16 << 20, MaxEvenOddClipWork: 10_000_000, MaxScanlineWork: 10_000_000,
		})
		if err := a.ctx.Err(); err != nil {
			return nil, err
		}
		if renderErr != nil {
			return nil, renderErr
		}
		data, err = d2raster.EncodePNG(a.ctx, frame)
	} else {
		config, _, decodeErr := image.DecodeConfig(bytes.NewReader(data))
		if err := a.ctx.Err(); err != nil {
			return nil, err
		}
		if decodeErr != nil {
			return nil, failure("unsupported_feature", "invalid raster image asset")
		}
		if config.Width <= 0 || config.Height <= 0 || config.Width > MaxAssetDimension || config.Height > MaxAssetDimension {
			return nil, failure("limit_exceeded", "image assets must be at most 1024 pixels per side")
		}
		a.pixels += config.Width * config.Height
		if a.pixels > 8<<20 {
			return nil, failure("limit_exceeded", "image assets exceed 8 million decoded pixels")
		}
		raster, _, err = image.Decode(bytes.NewReader(data))
		if err := a.ctx.Err(); err != nil {
			return nil, err
		}
		if err == nil {
			var normalized bytes.Buffer
			err = png.Encode(&normalized, raster)
			data = normalized.Bytes()
		}
	}
	if err := a.ctx.Err(); err != nil {
		return nil, err
	}
	if err != nil {
		return nil, failure("unsupported_feature", "cannot rasterize image asset")
	}
	if len(data) > MaxAssetBytes {
		return nil, failure("limit_exceeded", "normalized image asset exceeds 1 MiB")
	}
	result, err := url.Parse("data:image/png;base64," + base64.StdEncoding.EncodeToString(data))
	if err != nil {
		return nil, err
	}
	a.cache[key] = result
	return result, nil
}

func (a *assetLoader) embed(diagram *d2target.Diagram) error {
	convert := func(icon **url.URL) error {
		if *icon == nil {
			return nil
		}
		resolved, err := a.load(*icon)
		if err == nil {
			*icon = resolved
		}
		return err
	}
	if err := convert(&diagram.Root.Icon); err != nil {
		return err
	}
	for i := range diagram.Shapes {
		if err := convert(&diagram.Shapes[i].Icon); err != nil {
			return err
		}
	}
	for i := range diagram.Connections {
		if err := convert(&diagram.Connections[i].Icon); err != nil {
			return err
		}
	}
	if diagram.Legend != nil {
		for i := range diagram.Legend.Shapes {
			if err := convert(&diagram.Legend.Shapes[i].Icon); err != nil {
				return err
			}
		}
		for i := range diagram.Legend.Connections {
			if err := convert(&diagram.Legend.Connections[i].Icon); err != nil {
				return err
			}
		}
	}
	return nil
}

// SVG repeats embedded data at each use, unlike the native scene's asset table.
// Reserve twice each occurrence for renderer paths that emit an icon twice, and
// leave half the artifact budget for the surrounding geometry, fonts, and text.
func checkSVGAssetExpansion(diagram *d2target.Diagram) error {
	total := 0
	add := func(icon *url.URL) {
		if icon != nil {
			total += 2 * (len(icon.Scheme) + 1 + len(icon.Opaque) + len(icon.Path))
		}
	}
	add(diagram.Root.Icon)
	for _, shape := range diagram.Shapes {
		add(shape.Icon)
	}
	for _, edge := range diagram.Connections {
		add(edge.Icon)
	}
	if diagram.Legend != nil {
		for _, shape := range diagram.Legend.Shapes {
			add(shape.Icon)
		}
		for _, edge := range diagram.Legend.Connections {
			add(edge.Icon)
		}
	}
	if total > MaxArtifactBytes/2 {
		return failure("limit_exceeded", "repeated SVG image assets exceed the 8 MiB expansion budget; use smaller assets, fewer repeats, or PNG")
	}
	return nil
}

type noAssetNetwork struct{}

func (noAssetNetwork) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("asset network access is disabled")
}

func rasterAssets() (*d2scenebuild.AssetOptions, error) {
	// The prepared diagram contains only normalized inline assets. The resolver
	// still requires a base directory; use a root so data-only PNG rendering does
	// not depend on an available working directory (including in js/wasm).
	resolver, err := imageasset.New(imageasset.Options{BaseDir: "/", HTTPClient: &http.Client{Transport: noAssetNetwork{}}, Limits: imageasset.Limits{
		MaxFetchedBytes: MaxAssetBytes, MaxEncodedBytes: MaxAssetBytes, MaxDecompressedBytes: MaxAssetBytes, MaxSVGBytes: MaxAssetBytes,
		MaxDecodedWidth: MaxAssetDimension, MaxDecodedHeight: MaxAssetDimension, MaxDecodedPixels: 1 << 20,
		MaxAssets: 32, MaxCumulativeEncodedBytes: 32 << 20, MaxCumulativeDecodedBytes: 32 << 20,
	}})
	return &d2scenebuild.AssetOptions{Resolver: resolver,
		SVGImportLimits: d2svgimport.Limits{
			MaxBytes: 1 << 20, MaxDepth: 64, MaxElements: 10000, MaxAttributes: 20000, MaxAttributeBytes: 1 << 20,
			MaxPathCommands: 100000, MaxTransformFunctions: 10000, MaxUseDepth: 32, MaxResources: 10000,
		},
		SVGImportBudget: d2scenebuild.SVGImportBudget{
			MaxSourceBytes: 8 << 20, MaxElements: 100000, MaxAttributes: 100000, MaxAttributeBytes: 8 << 20,
			MaxPathCommands: 1000000, MaxTransformFunctions: 100000, MaxDeclaredResources: 100000, MaxExpandedUseInstances: 100000,
		},
	}, err
}
