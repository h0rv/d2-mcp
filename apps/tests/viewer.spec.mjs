import { test, expect, svg, svgResult, textResult, sendInput, sendResult, resolveCall, callCount } from './harness.mjs';

test('standard handshake enables only advertised actions inside an opaque sandbox', async ({ page, viewer }) => {
  const frame = await viewer();
  await expect(frame.locator('#render')).toBeDisabled();
  await sendInput(page);
  await expect(frame.locator('#render')).toBeEnabled();
  await expect(frame.locator('#download-source')).toBeEnabled();
  const initialize = await page.evaluate(() => window.harness.messages.find((m) => m.method === 'ui/initialize'));
  expect(initialize.params.appInfo.name).toBe('d2-viewer');
  expect(initialize.params.protocolVersion).toBeTruthy();
  expect(await page.locator('#viewer').getAttribute('sandbox')).toBe('allow-scripts');
  expect(await frame.locator('body').evaluate(() => {
    try { void parent.document.body; return 'accessible'; } catch { return 'opaque'; }
  })).toBe('opaque');
  expect(await page.evaluate(() => window.harness.messages.filter((m) => m.method === 'ui/notifications/initialized').length)).toBe(1);
});

test('hosts without tool or download capabilities still receive an inert preview', async ({ page, viewer }) => {
  const frame = await viewer({});
  await sendInput(page);
  await sendResult(page, svgResult());
  await expect(frame.locator('#diagram')).toBeVisible();
  // Opaque iframes produce blob:null URLs, which the standard host CSP does
  // not allow. This regression requires the portable data: image path.
  await expect(frame.locator('#diagram')).toHaveAttribute('src', /^data:image\/svg\+xml/);
  await expect(frame.locator('#render')).toBeDisabled();
  await expect(frame.locator('#download-source')).toBeDisabled();
  await expect(frame.locator('#download-image')).toBeDisabled();
  await expect(frame.locator('#support')).toContainText('does not advertise interactive tool calls');
  await expect(frame.locator('#support')).toContainText('does not advertise downloads');
  expect(await page.evaluate(() => window.harness.calls.length)).toBe(0);
});

test('complete-source edits send only rendering arguments after an explicit click', async ({ page, viewer }) => {
  const frame = await viewer();
  await sendInput(page, { code: 'a -> b', layout: 'elk', format: 'svg', theme_id: 1, sketch: false,
    file_path: '/secret.d2', document_id: 'private', revision: 'secret', output_name: 'overwrite.svg', overwrite: true,
    board_path: ['one'], invented: 'drop me', padding: 20 });
  await frame.locator('#source').fill('users -> API\nAPI -> database');
  await frame.locator('#layout').selectOption('tala');
  await frame.locator('#format').selectOption('png');
  expect(await page.evaluate(() => window.harness.calls.length)).toBe(0);
  await frame.locator('#render').click();
  await callCount(page, 1);
  const call = await page.evaluate(() => window.harness.calls[0].params);
  expect(call.name).toBe('render-d2');
  expect(call.arguments).toEqual({
    code: 'users -> API\nAPI -> database', layout: 'tala', format: 'png', theme_id: 1, sketch: false,
    board_path: ['one'], padding: 20,
  });
  await expect(frame.locator('#render')).toBeDisabled();
  await expect(frame.locator('#cancel')).toBeEnabled();
  await resolveCall(page, 0, textResult('Rendered'));
  await expect(frame.locator('#ascii')).toHaveText('Rendered');
  await expect(frame.locator('#render')).toBeEnabled();
});

test('file and document inputs are not fetched, and source limits count UTF-8 bytes', async ({ page, viewer }) => {
  const frame = await viewer();
  await sendInput(page, { code: 'old -> diagram' });
  await expect(frame.locator('#source')).toHaveValue('old -> diagram');
  await sendInput(page, { file_path: '/private/source.d2', document_id: 'secret', revision: '1' });
  await expect(frame.locator('#source')).toHaveValue('');
  await expect(frame.locator('#status')).toContainText('not fetched');
  await expect(frame.locator('#render')).toBeDisabled();
  const tooManyBytes = 'é'.repeat(131_073); // Below textarea's character limit, above the UTF-8 byte limit.
  await frame.locator('#source').fill(tooManyBytes);
  await frame.locator('#render').click();
  await expect(frame.locator('#status')).toContainText('UTF-8 bytes');
  await frame.locator('#download-source').click();
  await expect(frame.locator('#status')).toContainText('256 KiB download limit');
  expect(await page.evaluate(() => window.harness.calls.length)).toBe(0);
  expect(await page.evaluate(() => window.harness.downloads.length)).toBe(0);
  await sendInput(page, { code: tooManyBytes });
  await expect(frame.locator('#source')).toHaveValue('');
  await expect(frame.locator('#render')).toBeDisabled();
});

test('cancel and repeated renders ignore late results and honor host cancellation', async ({ page, viewer }) => {
  const frame = await viewer();
  await sendInput(page);
  await frame.locator('#render').click();
  await callCount(page, 1);
  await frame.locator('#cancel').click();
  await expect(frame.locator('#status')).toContainText('Rendering cancelled');
  await expect.poll(() => page.evaluate(() => window.harness.messages.some((m) => m.method === 'notifications/cancelled'))).toBe(true);
  await frame.locator('#source').fill('new -> diagram');
  await frame.locator('#render').click();
  await callCount(page, 2);
  await resolveCall(page, 1, svgResult(svg('<rect width="30" height="20" fill="green"/>', 30, 20)));
  await expect(frame.locator('#status')).toContainText('Preview ready');
  await resolveCall(page, 0, textResult('STALE RESPONSE'));
  await expect(frame.locator('#diagram')).toBeVisible();
  await expect(frame.locator('#ascii')).toBeHidden();
  await expect(frame.locator('#status')).toContainText('Preview ready');
  await page.evaluate(() => window.harness.cancel());
  await expect(frame.locator('#diagram')).toBeHidden();
  await expect(frame.locator('#status')).toHaveText('Rendering cancelled.');
  await frame.locator('#render').click();
  await callCount(page, 3);
  await sendInput(page, { code: 'host -> update' });
  await sendResult(page, textResult('A newer host result'));
  await resolveCall(page, 2, textResult('ANOTHER STALE RESPONSE'));
  await expect(frame.locator('#ascii')).toHaveText('A newer host result');
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

test('source and canonical PNG downloads use the host and handle decline and failure', async ({ page, viewer }) => {
  const frame = await viewer();
  await sendInput(page, { code: 'a -> b: <script>literal</script>' });
  await sendResult(page, svgResult(svg('<rect width="200" height="100" fill="red"/><script>alert(1)</script>')));
  await expect(frame.locator('#diagram')).toBeVisible();
  await frame.locator('#download-source').click();
  await expect.poll(() => page.evaluate(() => window.harness.downloads.length)).toBe(1);
  expect(await page.evaluate(() => window.harness.downloads[0])).toEqual({ contents: [{ type: 'resource', resource: {
    uri: 'file:///diagram.d2', mimeType: 'text/plain', text: 'a -> b: <script>literal</script>',
  } }] });
  await frame.locator('#download-image').click();
  await expect.poll(() => page.evaluate(() => window.harness.downloads.length)).toBe(2);
  const resource = await page.evaluate(() => window.harness.downloads[1].contents[0].resource);
  expect(resource.uri).toBe('file:///diagram.png');
  expect(resource.mimeType).toBe('image/png');
  const png = Buffer.from(resource.blob, 'base64');
  expect([...png.subarray(0, 8)]).toEqual([137, 80, 78, 71, 13, 10, 26, 10]);
  expect(png.readUInt32BE(16)).toBe(200);
  expect(png.readUInt32BE(20)).toBe(100);
  expect(png.includes(Buffer.from('<script>'))).toBe(false);
  await page.evaluate(() => { window.harness.downloadResult = { isError: true }; });
  await frame.locator('#download-source').click();
  await expect(frame.locator('#status')).toContainText('declined or cancelled');
  await page.evaluate(() => { window.harness.downloadError = 'Host download unavailable'; });
  await frame.locator('#download-source').click();
  await expect(frame.locator('#status')).toContainText('Host download unavailable');
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
});

test('real Go server renders all layouts, PNG, browser export and structured errors', async ({ page, viewer, harnessServer }, testInfo) => {
  if (process.env.CI) expect(harnessServer.realServer, 'CI must set D2_MCP_TEST_BINARY to a built Go server.').toBe(true);
  test.skip(!harnessServer.realServer, 'Set D2_MCP_TEST_BINARY to exercise the actual Go MCP server.');
  test.setTimeout(120_000);
  expect(harnessServer.resource.mimeType).toBe('text/html;profile=mcp-app');
  const frame = await viewer(undefined, true);
  const source = 'direction: right\nusers: Users\napi: API\ndb: Database { shape: cylinder }\nusers -> api: request\napi -> db: query';
  await sendInput(page, { code: source });
  for (const layout of ['dagre', 'elk', 'tala']) {
    await frame.locator('#layout').selectOption(layout);
    await frame.locator('#render').click();
    await expect(frame.locator('#status')).toContainText('Preview ready', { timeout: 30_000 });
    await expect(frame.locator('#diagram')).toBeVisible();
    expect(await frame.locator('#diagram').evaluate((image) => image.naturalWidth)).toBeGreaterThan(10);
    await frame.locator('#download-image').click();
    await expect(frame.locator('#status')).toContainText('Download handed to the host');
  }
  await page.locator('#viewer').screenshot({ path: testInfo.outputPath('mcp-app-preview.png') });
  await frame.locator('#format').selectOption('png');
  await frame.locator('#render').click();
  await expect(frame.locator('#status')).toContainText('Preview ready', { timeout: 30_000 });
  await frame.locator('#download-image').click();
  await expect(frame.locator('#status')).toContainText('Download handed to the host');
  await frame.locator('#source').fill('broken: {');
  await frame.locator('#render').click();
  await expect(frame.locator('#status')).toHaveAttribute('data-error', 'true');
  await expect(frame.locator('#diagram')).toBeHidden();
  await expect(frame.locator('#render')).toBeEnabled();
  expect(await page.evaluate(() => window.harness.downloads.length)).toBe(4);
});
