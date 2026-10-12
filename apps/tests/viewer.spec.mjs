import { test, expect, svg, svgResult, textResult, sendInput, sendResult } from './harness.mjs';

test('standard handshake permits viewing without tool capabilities in an opaque sandbox', async ({ page, viewer }) => {
  const frame = await viewer();
  await expect(frame.locator('#copy-source')).toBeDisabled();
  await sendInput(page);
  await expect(frame.locator('#copy-source')).toBeEnabled();
  const initialize = await page.evaluate(() => window.harness.messages.find((m) => m.method === 'ui/initialize'));
  expect(initialize.params.appInfo.name).toBe('d2-viewer');
  expect(initialize.params.protocolVersion).toBeTruthy();
  expect(await page.locator('#viewer').getAttribute('sandbox')).toBe('allow-scripts');
  expect(await frame.locator('body').evaluate(() => {
    try { void parent.document.body; return 'accessible'; } catch { return 'opaque'; }
  })).toBe('opaque');
  expect(await page.evaluate(() => window.harness.messages.filter((m) => m.method === 'ui/notifications/initialized').length)).toBe(1);
  await expect(frame.locator('#render, #cancel, select, [contenteditable=true]')).toHaveCount(0);
});

test('hosts without download capability still display an inert preview and source', async ({ page, viewer }) => {
  const frame = await viewer({});
  await sendInput(page);
  await sendResult(page, svgResult());
  await expect(frame.locator('#diagram')).toBeVisible();
  // Standard host CSP need not allow blob: URLs in an opaque iframe.
  await expect(frame.locator('#diagram')).toHaveAttribute('src', /^data:image\/svg\+xml/);
  await expect(frame.locator('#copy-source')).toBeEnabled();
  await expect(frame.locator('#download-image')).toBeHidden();
  await expect(frame.locator('#download-image')).toBeDisabled();
  expect(await page.evaluate(() => window.harness.calls.length)).toBe(0);
});

test('Copy D2 writes the exact source with the requested clipboard permission', async ({ page, context, viewer, harnessServer }) => {
  // Model a browser that grants the host's delegated clipboard-write request.
  // Keep the separate denial test in the same opaque sandbox without a grant.
  await context.grantPermissions(['clipboard-write']);
  const frame = await viewer(undefined, { clipboardWrite: {} });
  const source = 'users -> API: café\nAPI -> database';
  await sendInput(page, { code: source, theme_id: 6, layout: 'tala', file_path: '/private', output_name: 'private.svg' });
  await expect(frame.locator('#source-panel')).not.toHaveAttribute('open');
  await frame.locator('summary').click();
  await expect(frame.locator('#source')).toHaveAttribute('readonly', '');
  await frame.locator('#source').focus();
  await page.keyboard.type('accidental edit');
  await expect(frame.locator('#source')).toHaveValue(source);
  await frame.locator('#copy-source').click();
  await expect(frame.locator('#status')).toHaveText('D2 source copied.');
  // Grant test-only clipboard read after the user-triggered write. An earlier
  // origin-scoped grant changes Chromium's permission state for the iframe.
  await context.grantPermissions(['clipboard-read'], { origin: harnessServer.url });
  expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(source);
  expect(await page.evaluate(() => window.harness.calls)).toEqual([]);
});

test('clipboard denial selects read-only source for manual copying', async ({ page, viewer }) => {
  const frame = await viewer();
  const source = 'a -> b: π';
  await sendInput(page, { code: source });
  await frame.locator('#copy-source').click();
  await expect(frame.locator('#status')).toContainText('Press Ctrl+C or ⌘C');
  await expect(frame.locator('#source')).toBeFocused();
  expect(await frame.locator('#source').evaluate((el) => el.value.slice(el.selectionStart, el.selectionEnd))).toBe(source);
  expect(await page.evaluate(() => window.harness.calls)).toEqual([]);
});

test('file and document inputs are not fetched, and source limits count UTF-8 bytes', async ({ page, viewer }) => {
  const frame = await viewer();
  await sendInput(page, { code: 'old -> diagram' });
  await expect(frame.locator('#source')).toHaveValue('old -> diagram');
  for (const args of [{ file_path: '/private/source.d2', document_id: 'secret', revision: '1' }, { code: 'é'.repeat(131_073) }]) {
    await sendInput(page, args);
    await expect(frame.locator('#source')).toHaveValue('');
    await expect(frame.locator('#source-panel')).toBeHidden();
    await expect(frame.locator('#copy-source')).toBeDisabled();
  }
  await expect(frame.locator('#status')).toContainText('not fetched');
  expect(await page.evaluate(() => window.harness.calls.length)).toBe(0);
  expect(await page.evaluate(() => window.harness.downloads.length)).toBe(0);
});

test('new host input and cancellation discard an earlier image decode', async ({ page, viewer }) => {
  const frame = await viewer();
  await sendInput(page);
  // Hold the browser's real decode at its promise boundary to force the race.
  await frame.locator('body').evaluate(() => {
    const decode = HTMLImageElement.prototype.decode;
    HTMLImageElement.prototype.decode = function () {
      return decode.call(this).then(() => new Promise((resolve) => { globalThis.finishDecode = resolve; }));
    };
    globalThis.restoreDecode = () => { HTMLImageElement.prototype.decode = decode; };
  });
  await sendResult(page, svgResult());
  await expect.poll(() => frame.locator('body').evaluate(() => typeof globalThis.finishDecode)).toBe('function');
  await sendInput(page, { code: 'new -> diagram' });
  await sendResult(page, textResult('New host result'));
  await expect(frame.locator('#ascii')).toHaveText('New host result');
  await frame.locator('body').evaluate(() => { globalThis.restoreDecode(); globalThis.finishDecode(); });
  await expect(frame.locator('#diagram')).toBeHidden();
  await expect(frame.locator('#ascii')).toHaveText('New host result');
  await sendResult(page, svgResult());
  await expect(frame.locator('#diagram')).toBeVisible();
  await page.evaluate(() => window.harness.cancel());
  await expect(frame.locator('#diagram')).toBeHidden();
  await expect(frame.locator('#status')).toHaveText('Rendering cancelled.');
});

test('source, ASCII and structured errors remain literal text', async ({ page, viewer }) => {
  const frame = await viewer();
  const payload = '<img src="https://attacker.invalid/x" onerror="globalThis.compromised=true"><script>globalThis.compromised=true</script>';
  await sendInput(page, { code: payload });
  await expect(frame.locator('#source')).toHaveValue(payload);
  await sendResult(page, textResult(payload));
  await expect(frame.locator('#ascii')).toHaveText(payload);
  expect(await frame.locator('#ascii *').count()).toBe(0);
  await sendResult(page, { isError: true, content: [{ type: 'text', text: 'Fallback error' }], structuredContent: { error: { message: payload } } });
  await expect(frame.locator('#status')).toHaveText(payload);
  expect(await frame.locator('#status *').count()).toBe(0);
  expect(await frame.locator('body').evaluate(() => globalThis.compromised)).toBeUndefined();
});

test('hostile SVG stays an inert image without script execution or external requests', async ({ page, viewer }) => {
  const external = [];
  page.on('request', (request) => { if (request.url().includes('attacker.invalid')) external.push(request.url()); });
  const frame = await viewer();
  await sendInput(page);
  const hostile = svg(`<script>parent.postMessage({compromised:true},'*')</script>
    <style>@import url(https://attacker.invalid/import); rect{fill:url(https://attacker.invalid/fill)}</style>
    <image href="https://attacker.invalid/image" width="20" height="20"/>
    <foreignObject width="100" height="50"><div xmlns="http://www.w3.org/1999/xhtml"><img src="https://attacker.invalid/html" onerror="parent.postMessage({compromised:true},'*')"/></div></foreignObject>
    <rect width="100" height="50" onload="parent.postMessage({compromised:true},'*')"/>`);
  await sendResult(page, svgResult(hostile));
  await expect(frame.locator('#diagram')).toBeVisible();
  expect(await frame.locator('svg, foreignObject, iframe, object, embed').count()).toBe(0);
  expect(await page.evaluate(() => window.harness.messages.some((m) => m.compromised))).toBe(false);
  expect(external).toEqual([]);
  // The preview cannot mutate host HTML even if a result includes SVG HTML.
  expect(await page.locator('img, svg, foreignObject').count()).toBe(0);
});

test('cross-window spoofed messages and malformed parent messages cannot replace results', async ({ page, viewer, harnessServer }) => {
  const frame = await viewer();
  await sendInput(page);
  await sendResult(page, textResult('Trusted host result'));
  await expect(frame.locator('#ascii')).toHaveText('Trusted host result');
  await page.evaluate((url) => {
    const sibling = document.createElement('iframe');
    sibling.id = 'sibling'; sibling.src = `${url}/sibling`; document.body.append(sibling);
  }, harnessServer.url);
  const sibling = page.frameLocator('#sibling');
  await expect.poll(() => sibling.locator('html').evaluate(() => document.title)).toBe('Unrelated sibling frame');
  await sibling.locator('body').evaluate(() => {
    parent.frames[0].postMessage({ jsonrpc: '2.0', method: 'ui/notifications/tool-result', params: { content: [{ type: 'text', text: 'SPOOFED' }] } }, '*');
    parent.frames[0].postMessage({ jsonrpc: '2.0', method: 'ui/notifications/tool-input', params: { arguments: { code: 'SPOOFED SOURCE' } } }, '*');
  });
  await page.evaluate(() => {
    window.harness.raw({ jsonrpc: '1.0', method: 'ui/notifications/tool-result', params: { content: [{ type: 'text', text: 'MALFORMED' }] } });
    window.harness.raw('not JSON-RPC');
  });
  // A legitimate later notification acts as a message-order barrier.
  await page.evaluate(() => window.harness.raw({ jsonrpc: '2.0', method: 'ui/notifications/host-context-changed', params: { theme: 'dark' } }));
  await expect(frame.locator('html')).toHaveAttribute('data-theme', 'dark');
  await expect(frame.locator('#ascii')).toHaveText('Trusted host result');
  await expect(frame.locator('#source')).toHaveValue('a -> b');
});

test('malformed and oversized images fail safely, then a valid result recovers', async ({ page, viewer }) => {
  const frame = await viewer();
  await sendInput(page);
  const fakePNG = Buffer.alloc(33);
  Buffer.from([137, 80, 78, 71, 13, 10, 26, 10, 0, 0, 0, 13, 73, 72, 68, 82]).copy(fakePNG);
  fakePNG.writeUInt32BE(100_000, 16); fakePNG.writeUInt32BE(100_000, 20);
  const cases = [
    [svgResult('<!DOCTYPE svg [<!ENTITY x "hello">]>' + svg()), 'declarations'],
    [svgResult(svg('', 100_000, 100_000)), 'dimensions'],
    [svgResult('<svg xmlns="http://www.w3.org/2000/svg" width="NaN" height="20"/>'), 'dimensions'],
    [svgResult('<svg broken'), 'invalid SVG'],
    [svgResult(svg(' '.repeat(2 * 1024 * 1024))), '2 MiB'],
    [{ content: [{ type: 'image', mimeType: 'image/png', data: Buffer.from('not a PNG').toString('base64') }] }, 'invalid PNG'],
    [{ content: [{ type: 'image', mimeType: 'image/png', data: fakePNG.toString('base64') }] }, 'dimensions'],
    [{ content: [{ type: 'resource_link', uri: 'file:///private/diagram.svg', name: 'diagram.svg', mimeType: 'image/svg+xml' }] }, 'No inline preview'],
  ];
  for (const [result, message] of cases) {
    await sendResult(page, result);
    await expect(frame.locator('#status')).toContainText(message);
    await expect(frame.locator('#diagram')).toBeHidden();
    await expect(frame.locator('#download-image')).toBeDisabled();
  }
  await sendResult(page, svgResult());
  await expect(frame.locator('#diagram')).toBeVisible();
});

test('canonical PNG downloads use the host and handle decline, failure and late completion', async ({ page, viewer }) => {
  const frame = await viewer();
  await sendInput(page);
  await sendResult(page, svgResult(svg('<rect width="200" height="100" fill="red"/><script>alert(1)</script>')));
  await expect(frame.locator('#diagram')).toBeVisible();
  await frame.locator('#download-image').click();
  await expect.poll(() => page.evaluate(() => window.harness.downloads.length)).toBe(1);
  const resource = await page.evaluate(() => window.harness.downloads[0].contents[0].resource);
  expect(resource.uri).toBe('file:///diagram.png');
  expect(resource.mimeType).toBe('image/png');
  const png = Buffer.from(resource.blob, 'base64');
  expect([...png.subarray(0, 8)]).toEqual([137, 80, 78, 71, 13, 10, 26, 10]);
  expect(png.readUInt32BE(16)).toBe(200);
  expect(png.readUInt32BE(20)).toBe(100);
  expect(png.includes(Buffer.from('<script>'))).toBe(false);
  await page.evaluate(() => { window.harness.downloadResult = { isError: true }; });
  await frame.locator('#download-image').click();
  await expect(frame.locator('#status')).toContainText('declined or cancelled');
  await page.evaluate(() => { window.harness.downloadError = 'Host download unavailable'; });
  await frame.locator('#download-image').click();
  await expect(frame.locator('#status')).toContainText('Host download unavailable');
  await page.evaluate(() => { window.harness.deferDownload = true; });
  await frame.locator('#download-image').click();
  await expect.poll(() => page.evaluate(() => typeof window.harness.finishDownload)).toBe('function');
  await sendInput(page, { code: 'new -> diagram' });
  await sendResult(page, textResult('New host result'));
  await expect(frame.locator('#status')).toHaveText('Text result received.');
  await page.evaluate(() => window.harness.finishDownload({ isError: true }));
  await expect(frame.locator('#status')).toHaveText('Text result received.');
});

test('zoom, pan, fit and teardown operate on the current preview', async ({ page, viewer }) => {
  const frame = await viewer();
  await sendInput(page);
  await sendResult(page, svgResult());
  await expect(frame.locator('#diagram')).toBeVisible();
  const initial = await frame.locator('#diagram').getAttribute('style');
  await frame.locator('#zoom-in').click();
  expect(await frame.locator('#diagram').getAttribute('style')).not.toBe(initial);
  const viewport = await frame.locator('#viewport').boundingBox();
  await page.mouse.move(viewport.x + 50, viewport.y + 150);
  await page.mouse.down();
  await page.mouse.move(viewport.x + 90, viewport.y + 170);
  await page.mouse.up();
  expect(await frame.locator('#diagram').getAttribute('style')).toContain('40px');
  await frame.locator('#fit').click();
  expect(await frame.locator('#diagram').getAttribute('style')).toBe(initial);
  await page.evaluate(() => window.harness.teardown());
  await expect(frame.locator('#diagram')).toBeHidden();
  await expect(frame.locator('#diagram')).not.toHaveAttribute('src');
  await expect(frame.locator('#download-image')).toBeDisabled();
  await sendInput(page, { code: 'late -> input' });
  await sendResult(page, svgResult());
  await page.evaluate(() => window.harness.cancel());
  await expect(frame.locator('#source')).toHaveValue('');
  await expect(frame.locator('#source-panel')).toBeHidden();
  await expect(frame.locator('#copy-source')).toBeDisabled();
  await expect(frame.locator('#status')).toHaveText('Viewer closed.');
  await expect(frame.locator('#diagram')).not.toHaveAttribute('src');
});

test('real Go server artifacts preserve D2 themes across layouts, PNG, ASCII and errors', async ({ page, viewer, harnessServer }, testInfo) => {
  if (process.env.CI) expect(harnessServer.realServer, 'CI must set D2_MCP_TEST_BINARY to a built Go server.').toBe(true);
  test.skip(!harnessServer.realServer, 'Set D2_MCP_TEST_BINARY to exercise the actual Go MCP server.');
  test.setTimeout(120_000);
  expect(harnessServer.resource.mimeType).toBe('text/html;profile=mcp-app');
  expect(harnessServer.resource._meta.ui.permissions).toEqual({ clipboardWrite: {} });
  const frame = await viewer();
  const source = 'direction: right\nusers: Users\napi: API\ndb: Database { shape: cylinder }\nusers -> api: request\napi -> db: query';
  for (const layout of ['dagre', 'elk', 'tala']) {
    const result = await page.evaluate((args) => window.harness.render(args), { code: source, layout, format: 'svg', sketch: false, theme_id: 0 });
    await expect(frame.locator('#status')).toContainText('Preview ready', { timeout: 30_000 });
    await expect(frame.locator('#diagram')).toBeVisible();
    expect(await frame.locator('#diagram').evaluate((image) => image.naturalWidth)).toBeGreaterThan(10);
    const original = result.content.find((c) => c.type === 'resource').resource.text;
    // Parsing changes root sizing only; all D2 shapes, text, styles and colors survive.
    expect(await frame.locator('#diagram').evaluate((image, original) => {
      const parse = (s) => new DOMParser().parseFromString(s, 'image/svg+xml').documentElement.innerHTML;
      return parse(atob(image.src.split(',')[1])) === parse(original);
    }, original)).toBe(true);
    await frame.locator('#download-image').click();
    await expect(frame.locator('#status')).toContainText('Download handed to the host');
  }
  await frame.locator('body').screenshot({ path: testInfo.outputPath('mcp-app-preview.png') });
  const pngResult = await page.evaluate((args) => window.harness.render(args), { code: source, format: 'png', theme_id: 6 });
  await expect(frame.locator('#status')).toContainText('Preview ready', { timeout: 30_000 });
  const pngURL = `data:image/png;base64,${pngResult.content.find((c) => c.type === 'image').data}`;
  await expect(frame.locator('#diagram')).toHaveAttribute('src', pngURL);
  await page.evaluate(() => window.harness.raw({ jsonrpc: '2.0', method: 'ui/notifications/host-context-changed', params: { theme: 'dark' } }));
  await expect(frame.locator('html')).toHaveAttribute('data-theme', 'dark');
  await expect(frame.locator('#diagram')).toHaveAttribute('src', pngURL);
  expect(await frame.locator('#diagram').evaluate((image) => getComputedStyle(image).filter)).toBe('none');
  await frame.locator('#download-image').click();
  await expect(frame.locator('#status')).toContainText('Download handed to the host');
  await page.evaluate((args) => window.harness.render(args), { code: 'a -> b', format: 'ascii' });
  await expect(frame.locator('#ascii')).toContainText('a');
  await expect(frame.locator('#diagram')).toBeHidden();
  await page.evaluate((args) => window.harness.render(args), { code: 'broken: {', format: 'svg' });
  await expect(frame.locator('#status')).toHaveAttribute('data-error', 'true');
  await expect(frame.locator('#diagram')).toBeHidden();
  expect(await page.evaluate(() => window.harness.downloads.length)).toBe(4);
  expect(await page.evaluate(() => window.harness.calls)).toEqual([]);
});
