import wasm from './build/d2.wasm';
import './build/wasm_exec.js';

let ready;
function initialize() {
  return ready ??= (async () => {
    const go = new Go();
    const instance = await WebAssembly.instantiate(wasm, go.importObject);
    void go.run(instance);
    if (!globalThis.d2MCP) throw new Error('Go MCP handler did not register');
  })();
}

// Bound the body before copying it into Go; the Go handler validates JSON/MCP.
async function readBody(request) {
  if (!request.body) return '';
  const reader = request.body.getReader();
  const decoder = new TextDecoder();
  let body = '', size = 0;
  try {
    while (true) {
      const {value, done} = await reader.read();
      if (done) return body + decoder.decode();
      size += value.byteLength;
      if (size > 2 * 1024 * 1024) throw new RangeError('MCP request exceeds 2 MiB');
      body += decoder.decode(value, {stream: true});
    }
  } finally { await reader.cancel(); }
}

export class Renderer {
  // Compute only: no documents, sessions or storage reads/writes.
  async fetch(request) {
    let body;
    try { body = await readBody(request); }
    catch (error) { return new Response(error.message, {status: error instanceof RangeError ? 413 : 400}); }
    await initialize();
    const result = JSON.parse(await d2MCP(JSON.stringify({
      method: request.method, url: request.url, headers: Object.fromEntries(request.headers), body,
    }), request.signal));
    const headers = new Headers();
    for (const [name, values] of Object.entries(result.headers)) for (const value of values) headers.append(name, value);
    return new Response(result.status === 204 ? null : result.body, {status: result.status, headers});
  }
}

export default {
  async fetch(request, env) {
    if (new URL(request.url).pathname !== '/mcp') return new Response('Not found', {status: 404});
    if (!env.D2_MCP_TOKEN) return new Response('Configure D2_MCP_TOKEN before use', {status: 503});
    const encoder = new TextEncoder();
    const supplied = encoder.encode(request.headers.get('Authorization') ?? '');
    const expected = encoder.encode(`Bearer ${env.D2_MCP_TOKEN}`);
    if (supplied.length !== expected.length || !crypto.subtle.timingSafeEqual(supplied, expected)) return new Response('Unauthorized', {status: 401});
    return env.RENDERER.getByName('renderer').fetch(request);
  },
};
