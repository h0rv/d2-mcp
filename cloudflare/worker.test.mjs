import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';

import { Miniflare, convertV4MiniflareOptions } from 'miniflare';

test('generic Go Wasm bridge runs without a Cloudflare host', async () => {
  const { readFile } = await import('node:fs/promises');
  await import('./build/wasm_exec.js');
  const go = new Go();
  const { instance } = await WebAssembly.instantiate(
    await readFile('build/d2.wasm'),
    go.importObject,
  );
  void go.run(instance);
  const reply = JSON.parse(
    await d2MCP(
      JSON.stringify({
        method: 'POST',
        url: 'https://standalone.test/mcp',
        headers: {
          'Content-Type': 'application/json',
          Accept: 'application/json, text/event-stream',
          'MCP-Protocol-Version': '2025-11-25',
        },
        body: JSON.stringify({
          jsonrpc: '2.0',
          id: 1,
          method: 'tools/call',
          params: { name: 'render-d2', arguments: { code: 'a -> b', format: 'png' } },
        }),
      }),
      new AbortController().signal,
    ),
  );
  assert.equal(reply.status, 200);
  const result = JSON.parse(reply.body).result;
  assert.equal(result.isError, undefined);
  assert.equal(
    Buffer.from(result.content.find((c) => c.type === 'image').data, 'base64')
      .subarray(0, 8)
      .toString('hex'),
    '89504e470d0a1a0a',
  );
});

const config = JSON.parse(readFileSync(new URL('./wrangler.jsonc', import.meta.url), 'utf8'));
const ratelimits = Object.fromEntries(
  config.ratelimits.map(({ name, ...settings }) => [name, settings]),
);

const options = (bindings = {}, limits = ratelimits, entry) =>
  convertV4MiniflareOptions({
    name: 'd2-mcp-test',
    modules: [
      ...(entry ? [entry] : []),
      { type: 'ESModule', path: 'worker.js' },
      { type: 'Text', path: 'site/index.html' },
      { type: 'Text', path: 'site/example.svg' },
      { type: 'Text', path: 'site/copy.js' },
      { type: 'ESModule', path: 'build/wasm_exec.js' },
      { type: 'CompiledWasm', path: 'build/d2.wasm' },
    ],
    compatibilityDate: '2026-10-12',
    compatibilityFlags: ['enable_request_signal', 'request_signal_passthrough'],
    durableObjects: { RENDERER: { className: 'Renderer', useSQLite: true } },
    bindings,
    ratelimits: limits,
  });

const token = 'local-test-fixture';
const headers = {
  'Content-Type': 'application/json',
  Accept: 'application/json, text/event-stream',
  'MCP-Protocol-Version': '2025-11-25',
};

test('landing routes are static and independent of MCP access, quota and rendering', async () => {
  const mf = new Miniflare(
    options(
      {},
      {},
      {
        type: 'ESModule',
        path: 'static-site-test.js',
        contents: `import worker from './worker.js';
        export { Renderer } from './worker.js';
        export default { fetch(request) {
          return worker.fetch(request, new Proxy({}, {
            get() { throw new Error('static routes must not read MCP bindings'); }
          }));
        }};`,
      },
    ),
  );
  try {
    for (const [path, contentType, filename] of [
      ['/', 'text/html; charset=utf-8', 'index.html'],
      ['/example.svg', 'image/svg+xml', 'example.svg'],
      ['/copy.js', 'text/javascript; charset=utf-8', 'copy.js'],
    ]) {
      const response = await mf.dispatchFetch(`https://d2.horv.co${path}?source=test`);
      assert.equal(response.status, 200);
      assert.equal(response.headers.get('Content-Type'), contentType);
      assert.equal(response.headers.get('X-Content-Type-Options'), 'nosniff');
      assert.equal(response.headers.get('X-Frame-Options'), 'DENY');
      assert.equal(await response.text(), readFileSync(`site/${filename}`, 'utf8'));
      const head = await mf.dispatchFetch(`https://d2.horv.co${path}`, {
        method: 'HEAD',
      });
      assert.equal(head.status, 200);
      assert.equal(head.headers.get('Content-Type'), contentType);
      assert.equal(await head.text(), '');
      for (const method of ['POST', 'PUT', 'DELETE', 'OPTIONS']) {
        const rejected = await mf.dispatchFetch(`https://d2.horv.co${path}`, {
          method,
        });
        assert.equal(rejected.status, 405);
        assert.equal(rejected.headers.get('Allow'), 'GET, HEAD');
      }
    }
    for (const path of ['/missing', '/mcp/', '/mcp/anything', '/index.html', '/site/example.d2']) {
      const response = await mf.dispatchFetch(`https://d2.horv.co${path}`);
      assert.equal(response.status, 404);
      assert.equal(await response.text(), 'Not found');
    }
  } finally {
    await mf.dispose();
  }
});

for (const publicMode of [false, true]) {
  test(`${publicMode ? 'public' : 'private'} Go MCP: discovery, layouts, formats, errors and bounds`, async () => {
    const mf = new Miniflare(
      options(publicMode ? { D2_MCP_PUBLIC: 'true' } : { D2_MCP_TOKEN: token }),
    );
    const requestHeaders = publicMode ? headers : { ...headers, Authorization: `Bearer ${token}` };
    let id = 0;

    async function rpc(method, params, version = '2025-11-25') {
      const current = version === '2026-07-28';
      if (current)
        params = {
          ...params,
          _meta: {
            'io.modelcontextprotocol/protocolVersion': version,
            'io.modelcontextprotocol/clientCapabilities': {},
          },
        };
      const response = await mf.dispatchFetch('https://d2.test/mcp', {
        method: 'POST',
        headers: {
          ...requestHeaders,
          'MCP-Protocol-Version': version,
          'Mcp-Method': method,
          ...(params?.name ? { 'Mcp-Name': params.name } : {}),
        },
        body: JSON.stringify({ jsonrpc: '2.0', id: ++id, method, params }),
      });
      assert.equal(response.status, 200);
      assert.equal(response.headers.get('mcp-session-id'), null);
      const message = await response.json();
      assert.equal(message.error, undefined, JSON.stringify(message));
      return message.result;
    }

    try {
      assert.equal((await mf.dispatchFetch('https://d2.test/mcp')).status, publicMode ? 405 : 401);
      assert.equal(
        (await mf.dispatchFetch('https://d2.test/other', { headers: requestHeaders })).status,
        404,
      );
      assert.equal(
        (await mf.dispatchFetch('https://d2.test/mcp', { headers: requestHeaders })).status,
        405,
      );
      const init = await rpc('initialize', {
        protocolVersion: '2025-11-25',
        capabilities: {},
        clientInfo: { name: 'integration-test', version: '1' },
      });
      assert.equal(init.serverInfo.name, 'd2-mcp');
      const list = await rpc('tools/list', {}, '2026-07-28');
      assert.equal(list.tools.length, 3);
      const example = await rpc('tools/call', {
        name: 'render-d2',
        arguments: {
          code: readFileSync('site/example.d2', 'utf8').trimEnd(),
          format: 'svg',
        },
      });
      assert.equal(example.isError, undefined);
      assert.equal(
        example.content.find((c) => c.type === 'resource').resource.text,
        readFileSync('site/example.svg', 'utf8'),
      );
      assert.equal(example.structuredContent.options.theme_id, 0);
      assert.equal(example.structuredContent.options.padding, 100);
      assert.equal(example.structuredContent.options.sketch, false);
      for (const [layout, format] of [
        ['dagre', 'svg'],
        ['elk', 'png'],
        ['tala', 'ascii'],
      ]) {
        const result = await rpc('tools/call', {
          name: 'render-d2',
          arguments: { code: 'client -> api -> database', layout, format },
        });
        assert.equal(result.isError, undefined, JSON.stringify(result));
        assert.equal(result.structuredContent.options.layout, layout);
        assert.ok(result.structuredContent.artifact.bytes > 20);
        if (format === 'png')
          assert.equal(
            Buffer.from(result.content.find((c) => c.type === 'image').data, 'base64')
              .subarray(0, 8)
              .toString('hex'),
            '89504e470d0a1a0a',
          );
        if (format === 'svg')
          assert.match(result.content.find((c) => c.type === 'resource').resource.text, /<svg/);
      }
      for (const args of [
        { code: 'a: {' },
        { file_path: 'secret.d2' },
        { code: 'a.icon: https://example.com/image.png' },
      ]) {
        const result = await rpc('tools/call', { name: 'render-d2', arguments: args });
        assert.equal(result.isError, true);
      }
      assert.equal(
        (
          await mf.dispatchFetch('https://d2.test/mcp', {
            method: 'POST',
            headers: requestHeaders,
            body: '[]',
          })
        ).status,
        400,
      );
      assert.equal(
        (
          await mf.dispatchFetch('https://d2.test/mcp', {
            method: 'POST',
            headers: requestHeaders,
            body: ' '.repeat(2 * 1024 * 1024 + 1),
          })
        ).status,
        413,
      );
      assert.equal(
        (
          await mf.dispatchFetch('https://d2.test/mcp', {
            method: 'POST',
            headers: { ...requestHeaders, Origin: 'https://other.test' },
            body: '{}',
          })
        ).status,
        403,
      );
      assert.equal(
        (
          await rpc(
            'tools/call',
            { name: 'compile-d2', arguments: { code: 'a -> b' } },
            '2026-07-28',
          )
        ).structuredContent.valid,
        true,
      );
    } finally {
      await mf.dispose();
    }
  });
}

test('missing, invalid and conflicting access configuration fails closed', async () => {
  for (const bindings of [
    {},
    { D2_MCP_PUBLIC: 'false' },
    { D2_MCP_PUBLIC: '' },
    { D2_MCP_PUBLIC: 'TRUE' },
    { D2_MCP_PUBLIC: true },
    { D2_MCP_PUBLIC: 'yes', D2_MCP_TOKEN: token },
    { D2_MCP_PUBLIC: 'true', D2_MCP_TOKEN: token },
  ]) {
    const mf = new Miniflare(options(bindings));
    try {
      const response = await mf.dispatchFetch('https://d2.test/mcp', {
        headers: { ...headers, Authorization: `Bearer ${token}` },
      });
      assert.equal(response.status, 503, JSON.stringify(bindings));
    } finally {
      await mf.dispose();
    }
  }
});

test('explicit private mode preserves bearer checks without a public limiter', async () => {
  const mf = new Miniflare(options({ D2_MCP_PUBLIC: 'false', D2_MCP_TOKEN: token }, {}));
  try {
    for (const authorization of ['', 'Bearer wrong', `Bearer ${token.slice(1)}x`]) {
      const response = await mf.dispatchFetch('https://d2.test/mcp', {
        headers: { Authorization: authorization },
      });
      assert.equal(response.status, 401);
    }
    const response = await mf.dispatchFetch('https://d2.test/mcp', {
      headers: { Authorization: `Bearer ${token}` },
    });
    assert.equal(response.status, 405); // Authenticated request reaches the stateless Go handler.
  } finally {
    await mf.dispose();
  }
});

test('all public callers share the configured native request allowance', async () => {
  const mf = new Miniflare(options({ D2_MCP_PUBLIC: 'true' }));
  try {
    assert.deepEqual(ratelimits.PUBLIC_REQUESTS.simple, { limit: 60, period: 60 });
    assert.equal((await mf.dispatchFetch('https://d2.test/other')).status, 404);
    let limited;
    // The native limiter uses wall-clock windows; allow one boundary during the burst.
    for (let i = 0; i <= 120; i++) {
      const response = await mf.dispatchFetch(`https://d2.test/mcp?caller=${i}`, {
        headers: { 'CF-Connecting-IP': `192.0.2.${i}`, Authorization: `Bearer caller-${i}` },
      });
      if (response.status === 429) {
        limited = response;
        break;
      }
      assert.equal(response.status, 405); // Invalid GET still consumes public allowance.
    }
    assert.ok(limited, 'the shared public allowance must reject the request burst');
    assert.equal(limited.headers.get('Retry-After'), '60');
  } finally {
    await mf.dispose();
  }
});

test('missing, failing or denying limiter never forwards to rendering', async () => {
  const mf = new Miniflare(
    options(
      { D2_MCP_PUBLIC: 'true' },
      {},
      {
        type: 'ESModule',
        path: 'limiter-failure-test.js',
        contents: `import worker from './worker.js';
      export { Renderer } from './worker.js';
      export default {
        fetch(request, env) {
          const bindings = { ...env,
            RENDERER: { getByName() { throw new Error('renderer must not be reached'); } }
          };
          if (request.headers.has('X-Test-Throw'))
            bindings.PUBLIC_REQUESTS = { limit() { throw new Error('unavailable'); } };
          if (request.headers.has('X-Test-Deny'))
            bindings.PUBLIC_REQUESTS = { limit() { return { success: false }; } };
          return worker.fetch(request, bindings);
        }
      };`,
      },
    ),
  );
  try {
    for (const requestHeaders of [{}, { 'X-Test-Throw': '1' }]) {
      const response = await mf.dispatchFetch('https://d2.test/mcp', { headers: requestHeaders });
      assert.equal(response.status, 503);
      assert.equal(await response.text(), 'Public request limiter unavailable');
    }
    const limited = await mf.dispatchFetch('https://d2.test/mcp', {
      method: 'POST',
      headers: { ...headers, 'X-Test-Deny': '1' },
      body: ' '.repeat(2 * 1024 * 1024 + 1),
    });
    assert.equal(limited.status, 429); // No renderer access or oversized-body processing.
    assert.equal(limited.headers.get('Retry-After'), '60');
  } finally {
    await mf.dispose();
  }
});

test('cancelled current-protocol requests produce no result and leave the bridge usable', async () => {
  // The test entry supplies an already-aborted signal, without a timing race.
  const mf = new Miniflare(
    convertV4MiniflareOptions({
      name: 'cancelled-bridge',
      compatibilityDate: '2026-10-12',
      compatibilityFlags: ['enable_request_signal', 'request_signal_passthrough'],
      modules: [
        { type: 'ESModule', path: 'cancellation-test-worker.js' },
        { type: 'ESModule', path: 'worker.js' },
        { type: 'Text', path: 'site/index.html' },
        { type: 'Text', path: 'site/example.svg' },
        { type: 'Text', path: 'site/copy.js' },
        { type: 'ESModule', path: 'build/wasm_exec.js' },
        { type: 'CompiledWasm', path: 'build/d2.wasm' },
      ],
    }),
  );
  const params = {
    name: 'compile-d2',
    arguments: { code: 'a -> b' },
    _meta: {
      'io.modelcontextprotocol/protocolVersion': '2026-07-28',
      'io.modelcontextprotocol/clientCapabilities': {},
    },
  };
  const request = {
    method: 'POST',
    headers: {
      ...headers,
      'MCP-Protocol-Version': '2026-07-28',
      'Mcp-Method': 'tools/call',
      'Mcp-Name': 'compile-d2',
    },
    body: JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'tools/call', params }),
  };
  try {
    const cancelled = await mf.dispatchFetch('https://d2.test/mcp', {
      ...request,
      headers: { ...request.headers, 'X-Test-Abort': '1' },
    });
    assert.equal(await cancelled.text(), ''); // SDK closes a cancelled transport without a result.
    const next = await mf.dispatchFetch('https://d2.test/mcp', request);
    assert.equal((await next.json()).result.structuredContent.valid, true);
  } finally {
    await mf.dispose();
  }
});
