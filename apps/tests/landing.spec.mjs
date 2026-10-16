import { test, expect } from '@playwright/test';
import { createServer } from 'node:http';
import { readFile } from 'node:fs/promises';
import { once } from 'node:events';

let server, origin;
const source = (
  await readFile(new URL('../../cloudflare/site/example.d2', import.meta.url), 'utf8')
).trimEnd();

test.beforeAll(async () => {
  const files = new Map();
  for (const [route, name, contentType] of [
    ['/', 'index.html', 'text/html; charset=utf-8'],
    ['/example.svg', 'example.svg', 'image/svg+xml'],
    ['/example.png', 'example.png', 'image/png'],
    ['/example.txt', 'example.txt', 'text/plain; charset=utf-8'],
    ['/example.d2', 'example.d2', 'text/plain; charset=utf-8'],
    ['/copy.js', 'copy.js', 'text/javascript; charset=utf-8'],
  ]) {
    files.set(route, {
      body: await readFile(new URL(`../../cloudflare/site/${name}`, import.meta.url)),
      contentType,
    });
  }
  // Exercise the exact static files. Worker route/access behavior is covered by
  // cloudflare/worker.test.mjs against workerd and the real Go Wasm server.
  server = createServer((request, response) => {
    const file = files.get(new URL(request.url, 'http://localhost').pathname);
    if (!file) return response.writeHead(404).end();
    response.writeHead(200, { 'Content-Type': file.contentType }).end(file.body);
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  origin = `http://127.0.0.1:${server.address().port}`;
});

test.afterAll(async () => {
  server?.closeAllConnections();
  if (server) await new Promise((resolve) => server.close(resolve));
});

for (const [name, viewport] of [
  ['desktop', { width: 1280, height: 900 }],
  ['mobile', { width: 390, height: 844 }],
  ['small-mobile', { width: 320, height: 640 }],
]) {
  test(`landing ${name}: readable layout, exact keyboard copying and static example`, async ({
    page,
    context,
  }, info) => {
    await page.setViewportSize(viewport);
    await context.grantPermissions(['clipboard-read', 'clipboard-write']);
    const errors = [],
      requests = [];
    page.on('pageerror', (error) => errors.push(error.message));
    page.on('console', (message) => {
      if (message.type() === 'error') errors.push(message.text());
    });
    page.on('request', (request) => requests.push(request.url()));
    await page.goto(origin);
    await expect(page.getByRole('heading', { level: 1 })).toHaveText('d2-mcp');
    await expect(page.locator('#example-source')).toHaveValue(source);
    await expect(page.locator('#remote-url')).toHaveText('https://d2.horv.co/mcp');
    await expect(page.getByRole('tablist')).toHaveCount(2);
    await expect(page.getByRole('tabpanel')).toHaveCount(2);
    await expect(page.locator('main')).not.toContainText('A little source');
    await expect(page.locator('main')).not.toContainText('go install');
    await expect(page.locator('#output-svg-panel .diagram')).toBeVisible();
    expect(
      await page.locator('#output-svg-panel .diagram').evaluate((image) => image.complete && image.naturalWidth > 0),
    ).toBe(true);
    const copy = async (button) => {
      const expected = await page
        .locator(`#${await button.getAttribute('data-copy')}`)
        .evaluate((element) => element instanceof HTMLTextAreaElement ? element.value : element.textContent);
      await button.focus();
      await page.keyboard.press('Enter');
      await expect(button).toHaveText('Copied');
      await expect(page.getByRole('status')).toContainText('copied.');
      expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(expected);
    };
    await copy(page.getByRole('button', { name: 'Copy D2 source', exact: true }));
    for (const group of await page.locator('[data-tabs]').all()) {
      for (const tab of await group.getByRole('tab').all()) {
        await tab.click();
        const panel = group.getByRole('tabpanel');
        await expect(panel).toHaveCount(1);
        await expect(panel).toHaveAttribute('aria-labelledby', await tab.getAttribute('id'));
        await expect(tab).toHaveAttribute('aria-selected', 'true');
        for (const button of await panel.getByRole('button', { name: /^Copy / }).all()) await copy(button);
        const picture = panel.locator('img');
        if (await picture.count()) expect(await picture.evaluate((image) => image.complete && image.naturalWidth > 0)).toBe(true);
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      }
      await group.getByRole('tab').first().click();
    }
    await copy(page.getByRole('button', { name: 'Copy URL for MCP', exact: true }));
    expect(await page.locator('#example-ascii').textContent()).toBe(
      await readFile(new URL('../../cloudflare/site/example.txt', import.meta.url), 'utf8'),
    );
    // Capture the initial state, without temporary copy feedback or focus rings.
    await page.reload();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(
      true,
    );
    await page.screenshot({
      path: info.outputPath(`${name}.png`),
      fullPage: true,
    });
    await page.emulateMedia({ colorScheme: 'dark' });
    await page.screenshot({
      path: info.outputPath(`${name}-dark.png`),
      fullPage: true,
    });
    expect(requests.every((url) => url.startsWith(origin))).toBe(true);
    expect(errors).toEqual([]);
  });
}

test('landing clipboard denial selects the exact command for manual copying', async ({ page }) => {
  await page.goto(origin);
  await page.evaluate(() =>
    Object.defineProperty(navigator, 'clipboard', {
      value: {
        writeText: async () => {
          throw new Error('denied');
        },
      },
    }),
  );
  await page.getByRole('tab', { name: 'Codex', exact: true }).click();
  await page.getByRole('button', { name: 'Copy command for Codex', exact: true }).click();
  await expect(page.getByRole('status')).toContainText('Clipboard unavailable');
  await expect(page.locator('#codex-command')).toBeFocused();
  expect(await page.evaluate(() => window.getSelection().toString())).toBe(
    'codex mcp add d2 --url https://d2.horv.co/mcp',
  );
});

test('landing content and downloads remain usable without JavaScript', async ({ browser }) => {
  const context = await browser.newContext({
    javaScriptEnabled: false,
    viewport: { width: 390, height: 844 },
  });
  try {
    const page = await context.newPage();
    await page.goto(origin);
    await expect(page.locator('#claude-command')).toContainText('https://d2.horv.co/mcp');
    await expect(page.locator('button[data-copy]:visible')).toHaveCount(0);
    await expect(page.locator('#output-svg-panel .diagram')).toBeVisible();
    await page.getByRole('link', { name: 'Downloads', exact: true }).click();
    await expect(
      page.getByRole('heading', { name: 'Local downloads', exact: true }),
    ).toBeInViewport();
    await expect(page.getByRole('tablist')).toHaveCount(0);
    await expect(page.getByRole('tabpanel')).toHaveCount(0);
    await expect(page.locator('[data-panel]:visible')).toHaveCount(6);
    await expect(page.getByRole('link', { name: 'Releases', exact: true })).toHaveAttribute(
      'href',
      'https://github.com/h0rv/d2-mcp/releases',
    );
    await expect(page.locator('#local')).toContainText(
      'Binaries for the updated server aren’t published yet.',
    );
  } finally {
    await context.close();
  }
});

for (const label of ['MCP client', 'Output format']) {
  test(`landing ${label} tabs support keyboard selection and roving focus`, async ({ page }) => {
    await page.goto(origin);
    const list = page.getByRole('tablist', { name: label });
    const tabs = list.getByRole('tab');
    const count = await tabs.count();
    const check = async (index) => {
      await expect(tabs.nth(index)).toBeFocused();
      await expect(tabs.nth(index)).toHaveAttribute('aria-selected', 'true');
      await expect(list.locator('[tabindex="0"]')).toHaveCount(1);
      await expect(page.locator(`#${await tabs.nth(index).getAttribute('aria-controls')}`)).toBeVisible();
    };
    await tabs.first().focus();
    await page.keyboard.press('ArrowLeft');
    await check(count - 1);
    await page.keyboard.press('ArrowRight');
    await check(0);
    await page.keyboard.press('End');
    await check(count - 1);
    await page.keyboard.press('Home');
    await check(0);
    await page.keyboard.press('ArrowRight');
    await check(1);
    await page.keyboard.press('Tab');
    const panel = page.locator(`#${await tabs.nth(1).getAttribute('aria-controls')}`);
    if (label === 'MCP client') await expect(panel.locator('code')).toBeFocused();
    else await expect(panel).toBeFocused();
    await expect(page.locator('[data-tabs]').filter({ has: list }).locator('[data-panel][hidden] :focus')).toHaveCount(0);
  });
}


test('source can be selected and copied with native keyboard commands', async ({ page, context }) => {
  await context.grantPermissions(['clipboard-read', 'clipboard-write']);
  await page.goto(origin);
  const input = page.getByRole('textbox', { name: 'D2 source', exact: true });
  await input.focus();
  await page.keyboard.press('ControlOrMeta+A');
  expect(await input.evaluate((element) => element.value.slice(element.selectionStart, element.selectionEnd))).toBe(source);
  await page.keyboard.press('ControlOrMeta+C');
  expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(source);
  await page.keyboard.type('accidental edit');
  await expect(input).toHaveValue(source);
});

test('setup text can be selected with the pointer independently of the copy script', async ({ page }) => {
  await page.goto(origin);
  await page.getByRole('tab', { name: 'Codex', exact: true }).click();
  const command = page.locator('#codex-command');
  await command.scrollIntoViewIfNeeded();
  const bounds = await command.boundingBox();
  await page.mouse.move(bounds.x + 1, bounds.y + bounds.height / 2);
  await page.mouse.down();
  await page.mouse.move(bounds.x + bounds.width - 1, bounds.y + bounds.height / 2, { steps: 15 });
  await page.mouse.up();
  expect(await page.evaluate(() => window.getSelection().toString())).toBe(await command.textContent());
});

for (const missing of [false, true]) {
  test(`mobile source copy fallback with ${missing ? 'missing' : 'denied'} clipboard`, async ({ browser }) => {
    const context = await browser.newContext({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true });
    try {
      const page = await context.newPage();
      await page.goto(origin);
      await page.evaluate((missing) => Object.defineProperty(navigator, 'clipboard', {
        value: missing ? undefined : { writeText: async () => { throw new Error('denied'); } },
      }), missing);
      await page.getByRole('button', { name: 'Copy D2 source', exact: true }).tap();
      await expect(page.getByRole('button', { name: 'Copy D2 source', exact: true })).toHaveText('Text selected');
      const input = page.locator('#example-source');
      await expect(input).toBeFocused();
      expect(await input.evaluate((element) => element.value.slice(element.selectionStart, element.selectionEnd))).toBe(source);
      await expect(page.getByRole('status')).toContainText('use your device’s Copy command');
    } finally { await context.close(); }
  });
}

test('example downloads return the exact checked-in source and artifacts', async ({ page }) => {
  await page.goto(origin);
  for (const [tab, link, filename] of [
    [null, 'Download source', 'example.d2'],
    ['SVG', 'Download SVG', 'example.svg'],
    ['PNG', 'Download PNG', 'example.png'],
    ['ASCII', 'Download ASCII', 'example.txt'],
  ]) {
    if (tab) await page.getByRole('tab', { name: tab, exact: true }).click();
    const pending = page.waitForEvent('download');
    await page.getByRole('link', { name: link, exact: true }).click();
    const download = await pending;
    expect(await download.failure()).toBeNull();
    expect(await readFile(await download.path())).toEqual(await readFile(new URL(`../../cloudflare/site/${filename}`, import.meta.url)));
  }
});

test('touch users can copy source and commands and switch output formats', async ({ browser }) => {
  const context = await browser.newContext({
    viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true,
    permissions: ['clipboard-read', 'clipboard-write'],
  });
  try {
    const page = await context.newPage();
    await page.goto(origin);
    await page.getByRole('button', { name: 'Copy D2 source', exact: true }).tap();
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(source);
    await page.getByRole('tab', { name: 'PNG', exact: true }).tap();
    await expect(page.locator('#output-png-panel .diagram')).toBeVisible();
    await page.getByRole('tab', { name: 'Codex', exact: true }).tap();
    await page.getByRole('button', { name: 'Copy command for Codex', exact: true }).tap();
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(
      await page.locator('#codex-command').textContent(),
    );
  } finally {
    await context.close();
  }
});
