//go:build js && wasm

// Local feasibility bridge, not an MCP server or a deployment entry point.
package main

import (
	"context"
	"encoding/json"
	"io/fs"
	"runtime"
	"syscall/js"

	"github.com/h0rv/d2-mcp/d2"
)

type noFiles struct{}

func (noFiles) Open(string) (fs.File, error) { return nil, fs.ErrPermission }

func main() {
	js.Global().Set("d2ProbeRender", js.FuncOf(func(_ js.Value, args []js.Value) any {
		input := args[0].String()
		executor := js.FuncOf(func(_ js.Value, callbacks []js.Value) any {
			resolve := callbacks[0]
			go func() {
				var request struct {
					Code   string `json:"code"`
					Layout string `json:"layout"`
					Format string `json:"format"`
				}
				result := map[string]any{}
				err := json.Unmarshal([]byte(input), &request)
				if err == nil {
					if request.Layout == "" {
						request.Layout = "dagre"
					}
					if request.Format == "" {
						request.Format = "svg"
					}
					var prepared *d2.Prepared
					prepared, err = d2.Prepare(context.Background(), d2.Source{Code: request.Code, Path: "input.d2", FS: noFiles{}}, d2.Options{Layout: &request.Layout})
					if err == nil {
						var artifact []byte
						artifact, err = d2.Render(context.Background(), prepared, request.Format, "extended")
						result["bytes"] = len(artifact)
						if request.Format == "svg" {
							result["svg"] = string(artifact)
						}
					}
				}
				if err != nil {
					result["error"] = err.Error()
				}
				var memory runtime.MemStats
				runtime.ReadMemStats(&memory)
				result["goMemory"] = map[string]any{"heapAlloc": memory.HeapAlloc, "heapSys": memory.HeapSys, "sys": memory.Sys, "numGC": memory.NumGC}
				encoded, _ := json.Marshal(result)
				resolve.Invoke(string(encoded))
			}()
			return nil
		})
		promise := js.Global().Get("Promise").New(executor)
		executor.Release()
		return promise
	}))
	select {}
}
