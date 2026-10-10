import wasm from './build/d2.wasm';
import './build/wasm_exec.js';

let ready;
let instance;
let initialMemory;
function initialize() {
  if (!ready) {
    ready = (async () => {
      const go = new Go();
      instance = await WebAssembly.instantiate(wasm, go.importObject);
      initialMemory = instance.exports.mem.buffer.byteLength;
      // Go starts inside a request: its crypto/time APIs cannot run at module scope.
      void go.run(instance);
      if (!globalThis.d2ProbeRender) throw new Error('Go bridge did not register');
    })();
  }
  return ready;
}

async function render(request, kind) {
  await initialize();
  const result = JSON.parse(await d2ProbeRender(await request.text()));
  result.runtime = { kind, initialWasmMemoryBytes: initialMemory, wasmMemoryBytes: instance.exports.mem.buffer.byteLength };
  return Response.json(result, {status: result.error ? 422 : 200});
}

export class Renderer {
  // No storage reads or writes; only warm renderer code/memory is retained.
  async fetch(request) { return render(request, 'durable-object'); }
}

export default {
  async fetch(request, env) {
    if (new URL(request.url).pathname === '/do') {
      return env.RENDERER.getByName('local-renderer').fetch(request);
    }
    return render(request, 'worker');
  },
};
