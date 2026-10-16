import { test, expect } from '@playwright/test';
import { createServer } from 'node:http';
import { readFile } from 'node:fs/promises';
import { once } from 'node:events';

let server, origin;
test.beforeAll(async () => {
  const files = new Map();
  for (const [route, name, contentType] of [
    ['/', 'index.html', 'text/html; charset=utf-8'],
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
  test(`landing ${name}: readable setup, exact keyboard copying and prompt`, async ({
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
    await expect(page.getByRole('heading', { level: 2 }).first()).toHaveText('1. Connect your agent');
    await expect(page.locator('img, textarea, .example')).toHaveCount(0);
    await expect(page.locator('#remote-url')).toHaveText('https://d2.horv.co/mcp');
    await expect(page.getByRole('tablist')).toHaveCount(1);
    await expect(page.getByRole('tabpanel')).toHaveCount(1);
    await expect(page.locator('main')).not.toContainText('A little source');
    await expect(page.locator('main')).not.toContainText('go install');
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
    await copy(page.getByRole('button', { name: 'Copy prompt', exact: true }));
    for (const group of await page.locator('[data-tabs]').all()) {
      for (const tab of await group.getByRole('tab').all()) {
        await tab.click();
        const panel = group.getByRole('tabpanel');
        await expect(panel).toHaveCount(1);
        await expect(panel).toHaveAttribute('aria-labelledby', await tab.getAttribute('id'));
        await expect(tab).toHaveAttribute('aria-selected', 'true');
        for (const button of await panel.getByRole('button', { name: /^Copy / }).all()) await copy(button);
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      }
      await group.getByRole('tab').first().click();
    }
    await copy(page.getByRole('button', { name: 'Copy URL for MCP', exact: true }));
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
    await page.getByRole('tab', { name: 'ChatGPT', exact: true }).click();
    await page.screenshot({ path: info.outputPath(`${name}-chatgpt-dark.png`), fullPage: true });
    await page.emulateMedia({ colorScheme: 'light' });
    await page.screenshot({ path: info.outputPath(`${name}-chatgpt.png`), fullPage: true });
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

test('landing setup and prompt remain usable without JavaScript', async ({ browser }) => {
  const context = await browser.newContext({
    javaScriptEnabled: false,
    viewport: { width: 390, height: 844 },
  });
  try {
    const page = await context.newPage();
    await page.goto(origin);
    await expect(page.locator('#claude-command')).toContainText('https://d2.horv.co/mcp');
    await expect(page.locator('button[data-copy]:visible')).toHaveCount(0);
    await expect(page.locator('#agent-prompt')).toContainText('Validate the D2, render a PNG');
    await expect(page.getByRole('tablist')).toHaveCount(0);
    await expect(page.getByRole('tabpanel')).toHaveCount(0);
    await expect(page.locator('[data-panel]:visible')).toHaveCount(4);
    await expect(page.locator('#agent-chatgpt-panel')).toContainText('Add custom MCP server');
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  } finally {
    await context.close();
  }
});

for (const label of ['MCP client']) {
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
    await expect(panel.getByRole('link', { name: 'Plugins', exact: true })).toBeFocused();
    await expect(page.locator('[data-tabs]').filter({ has: list }).locator('[data-panel][hidden] :focus')).toHaveCount(0);
  });
}


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
  test(`mobile prompt copy fallback with ${missing ? 'missing' : 'denied'} clipboard`, async ({ browser }) => {
    const context = await browser.newContext({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true });
    try {
      const page = await context.newPage();
      await page.goto(origin);
      await page.evaluate((missing) => Object.defineProperty(navigator, 'clipboard', {
        value: missing ? undefined : { writeText: async () => { throw new Error('denied'); } },
      }), missing);
      await page.getByRole('button', { name: 'Copy prompt', exact: true }).tap();
      await expect(page.getByRole('button', { name: 'Copy prompt', exact: true })).toHaveText('Text selected');
      const prompt = page.locator('#agent-prompt');
      await expect(prompt).toBeFocused();
      expect(await page.evaluate(() => window.getSelection().toString())).toBe(await prompt.textContent());
      await expect(page.getByRole('status')).toContainText('use your device’s Copy command');
    } finally { await context.close(); }
  });
}

test('touch users can copy the prompt, switch clients and copy the endpoint', async ({ browser }) => {
  const context = await browser.newContext({
    viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true,
    permissions: ['clipboard-read', 'clipboard-write'],
  });
  try {
    const page = await context.newPage();
    await page.goto(origin);
    await page.getByRole('button', { name: 'Copy prompt', exact: true }).tap();
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(await page.locator('#agent-prompt').textContent());
    await page.getByRole('tab', { name: 'ChatGPT', exact: true }).tap();
    await expect(page.locator('#agent-chatgpt-panel')).toBeVisible();
    await page.getByRole('button', { name: 'Copy URL for MCP', exact: true }).tap();
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe('https://d2.horv.co/mcp');
    await page.getByRole('tab', { name: 'Codex', exact: true }).tap();
    await page.getByRole('button', { name: 'Copy command for Codex', exact: true }).tap();
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(
      await page.locator('#codex-command').textContent(),
    );
  } finally {
    await context.close();
  }
});


test('client instructions offer direct remote setup and a concrete reusable prompt', async ({ page }) => {
  await page.goto(origin);
  for (const [client, id, command] of [
    ['Claude Code', 'claude', 'claude mcp add --scope user --transport http d2 https://d2.horv.co/mcp'],
    ['Codex', 'codex', 'codex mcp add d2 --url https://d2.horv.co/mcp'],
    ['pi', 'pi', 'pi mcp add d2 --exposure direct --url https://d2.horv.co/mcp'],
  ]) {
    await page.getByRole('tab', { name: client, exact: true }).click();
    expect(await page.locator(`#${id}-command`).textContent()).toBe(command);
  }
  await page.getByRole('tab', { name: 'ChatGPT', exact: true }).click();
  const chatgpt = page.getByRole('tabpanel');
  await expect(chatgpt).toContainText('Add custom MCP server');
  await expect(chatgpt).toContainText('needs no authentication');
  await expect(chatgpt).toContainText('Requires an account or workspace');
  await expect(chatgpt).not.toContainText('Developer Mode');
  await expect(page.locator('main')).not.toContainText('marketplace');
  await expect(page.locator('#agent-prompt')).toContainText('Ask about missing details instead of inventing them.');
  await expect(page.locator('#agent-prompt')).toContainText('Validate the D2, render a PNG');
  await expect(page.locator('#agent-prompt')).toContainText('bottleneck or single point of failure');
});
