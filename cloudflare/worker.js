import wasm from './build/d2.wasm';
import './build/wasm_exec.js';
import page from './site/index.html';
import example from './site/example.svg';
import copyScript from './site/copy.js';

const staticFiles = new Map([
  ['/', [page, 'text/html; charset=utf-8']],
  ['/example.svg', [example, 'image/svg+xml']],
  ['/copy.js', [copyScript, 'text/javascript; charset=utf-8']],
]);

let ready;

function initialize() {
  return (ready ??= (async () => {
    const go = new Go();
    const instance = await WebAssembly.instantiate(wasm, go.importObject);
    void go.run(instance);

    if (!globalThis.d2MCP) throw new Error('Go MCP handler did not register');
  })());
}

// Bound the body before copying it into Go; the Go handler validates JSON/MCP.
async function readBody(request) {
  if (!request.body) return '';

  const reader = request.body.getReader();
  const decoder = new TextDecoder();
  let body = '',
    size = 0;

  try {
    while (true) {
      const { value, done } = await reader.read();
      if (done) return body + decoder.decode();
      size += value.byteLength;
      if (size > 2 * 1024 * 1024) throw new RangeError('MCP request exceeds 2 MiB');
      body += decoder.decode(value, { stream: true });
    }
  } finally {
    await reader.cancel();
  }
}

export class Renderer {
  // Compute only: no documents, sessions or storage reads/writes.
  async fetch(request) {
    let body;
    try {
      body = await readBody(request);
    } catch (error) {
      return new Response(error.message, { status: error instanceof RangeError ? 413 : 400 });
    }

    await initialize();
    const result = JSON.parse(
      await d2MCP(
        JSON.stringify({
          method: request.method,
          url: request.url,
          headers: Object.fromEntries(request.headers),
          body,
        }),
        request.signal,
      ),
    );

    const headers = new Headers();
    for (const [name, values] of Object.entries(result.headers))
      for (const value of values) headers.append(name, value);

    return new Response(result.status === 204 ? null : result.body, {
      status: result.status,
      headers,
    });
  }
}

export default {
  async fetch(request, env) {
    const pathname = new URL(request.url).pathname;
    const file = staticFiles.get(pathname);
    if (file) {
      if (!['GET', 'HEAD'].includes(request.method))
        return new Response('Method not allowed', {
          status: 405,
          headers: { Allow: 'GET, HEAD' },
        });

      return new Response(request.method === 'HEAD' ? null : file[0], {
        headers: {
          'Content-Type': file[1],
          'Cache-Control': 'public, max-age=300',
          'X-Content-Type-Options': 'nosniff',
          'X-Frame-Options': 'DENY',
          'Referrer-Policy': 'no-referrer',
        },
      });
    }
    if (pathname !== '/mcp') return new Response('Not found', { status: 404 });

    const publicMode = env.D2_MCP_PUBLIC === 'true';
    if (
      (env.D2_MCP_PUBLIC !== undefined && !['true', 'false'].includes(env.D2_MCP_PUBLIC)) ||
      (publicMode && env.D2_MCP_TOKEN)
    )
      return new Response('Configure either public access or a private bearer token', {
        status: 503,
      });

    if (publicMode) {
      try {
        const { success } = await env.PUBLIC_REQUESTS.limit({ key: 'd2-mcp:public:/mcp' });
        if (!success)
          return new Response('Public request limit reached; retry later', {
            status: 429,
            headers: { 'Retry-After': '60' },
          });
      } catch {
        return new Response('Public request limiter unavailable', { status: 503 });
      }
    } else {
      if (!env.D2_MCP_TOKEN)
        return new Response('Configure D2_MCP_TOKEN or explicitly set D2_MCP_PUBLIC=true', {
          status: 503,
        });

      const encoder = new TextEncoder();
      const supplied = encoder.encode(request.headers.get('Authorization') ?? '');
      const expected = encoder.encode(`Bearer ${env.D2_MCP_TOKEN}`);
      if (supplied.length !== expected.length || !crypto.subtle.timingSafeEqual(supplied, expected))
        return new Response('Unauthorized', { status: 401 });
    }

    return env.RENDERER.getByName('renderer').fetch(request);
  },
};
