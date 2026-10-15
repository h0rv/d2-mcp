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
    await expect(page.locator('#example-source')).toHaveText(source);
    await expect(page.locator('#remote-url')).toHaveText('https://d2.horv.co/mcp');
    await expect(page.locator('#install-command')).toContainText(
      'go install github.com/h0rv/d2-mcp@main',
    );
    await expect(
      page.getByRole('navigation', { name: 'Download version 1.2.0' }).getByRole('link'),
    ).toHaveCount(5);
    await expect(page.locator('.diagram')).toBeVisible();
    expect(
      await page.locator('.diagram').evaluate((image) => image.complete && image.naturalWidth > 0),
    ).toBe(true);
    for (const button of await page.locator('button[data-copy]').all()) {
      const expected = await page
        .locator(`#${await button.getAttribute('data-copy')}`)
        .textContent();
      await button.focus();
      await page.keyboard.press('Enter');
      await expect(button).toHaveText('Copied');
      await expect(page.getByRole('status')).toContainText('copied.');
      expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(expected);
    }
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
  await page.getByRole('button', { name: 'Copy Codex setup', exact: true }).click();
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
    await expect(page.locator('.diagram')).toBeVisible();
    await page.getByRole('link', { name: 'Use it locally', exact: true }).click();
    await expect(
      page.getByRole('heading', { name: 'Use it locally', exact: true }),
    ).toBeInViewport();
    await expect(page.getByRole('link', { name: 'Linux x64', exact: true })).toHaveAttribute(
      'href',
      'https://github.com/h0rv/d2-mcp/releases/download/1.2.0/d2-mcp-linux-amd64',
    );
  } finally {
    await context.close();
  }
});
