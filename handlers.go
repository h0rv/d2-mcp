package main

import (
	"context"
	"encoding/base64"
	"errors"

	d2 "github.com/h0rv/d2-mcp/d2"

	"github.com/mark3labs/mcp-go/mcp"
)

func CompileD2Handler(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	code, ok := request.Params.Arguments["code"].(string)
	if !ok {
		return nil, errors.New("code must be a string")
	}

	_, _, compileErr, otherErr := d2.Compile(ctx, code)
	if otherErr != nil {
		return nil, otherErr
	}

	if compileErr != nil {
		return mcp.NewToolResultError(compileErr.Error()), nil
	}

	return mcp.NewToolResultText("D2 script compiled successfully"), nil
}

func RenderD2Handler(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	code, ok := request.Params.Arguments["code"].(string)
	if !ok {
		return nil, errors.New("code must be a string")
	}

	svg, err := d2.Render(ctx, code)
	if err != nil {
		return nil, err
	}

	var (
		img     []byte
		imgType string
	)

	if GlobalImageType == "png" {
		png, err := SvgToPng(ctx, svg)
		if err != nil {
			return nil, err
		}
		img = png
		imgType = "image/png"
	} else if GlobalImageType == "svg" {
		img = svg
		imgType = "image/svg+xml"
	} else {
		return nil, errors.New("invalid image type: " + GlobalImageType)
	}

	imageEncoded := base64.StdEncoding.EncodeToString(img)

	return mcp.NewToolResultImage("D2 diagram", imageEncoded, imgType), nil
}
