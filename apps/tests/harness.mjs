import { test as base, expect } from '@playwright/test';
import { build } from 'esbuild';

import { createServer } from 'node:http';
import { readFile } from 'node:fs/promises';
import { once } from 'node:events';
import { fileURLToPath } from 'node:url';

export const test = base.extend({
  harnessServer: [
    async ({}, use) => {
      const host = await build({
        entryPoints: [fileURLToPath(new URL('./host.js', import.meta.url))],
        bundle: true,
        write: false,
        format: 'iife',
        platform: 'browser',
        target: 'es2022',
      });
      let client, resource;
      if (process.env.D2_MCP_TEST_BINARY) {
        const { Client } = await import('@modelcontextprotocol/client');
        const { StdioClientTransport } = await import('@modelcontextprotocol/client/stdio');
        client = new Client(
          { name: 'd2-browser-tests', version: '1.0.0' },
          {
            capabilities: {
              extensions: {
                'io.modelcontextprotocol/ui': { mimeTypes: ['text/html;profile=mcp-app'] },
              },
            },
          },
        );
        await client.connect(
          new StdioClientTransport({
            command: process.env.D2_MCP_TEST_BINARY,
            args: ['--ui', '--transport', 'stdio'],
            stderr: 'pipe',
          }),
        );
        resource = (await client.readResource({ uri: 'ui://d2/viewer' })).contents[0];
      }
      const viewer =
        resource?.text || (await readFile(new URL('../dist/viewer.html', import.meta.url), 'utf8'));
      const server = createServer(async (request, response) => {
        try {
          if (request.url === '/host.js') {
            response.setHeader('Content-Type', 'text/javascript');
            response.end(host.outputFiles[0].text);
          } else if (request.url === '/viewer.html') {
            response.setHeader('Content-Type', 'text/html');
            // Model the host's empty ui.csp allowlists. The viewer also supplies a
            // stricter hash-based script policy in its own HTML resource.
            response.setHeader(
              'Content-Security-Policy',
              "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src 'self' data:; font-src 'none'; connect-src 'none'; frame-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'",
            );
            response.end(viewer);
          } else if (request.url === '/tools/call' && request.method === 'POST' && client) {
            let body = '';
            for await (const chunk of request) {
              body += chunk;
              if (body.length > 1024 * 1024) throw new Error('Harness request is too large');
            }
            const result = await client.callTool(JSON.parse(body));
            response.setHeader('Content-Type', 'application/json');
            response.end(JSON.stringify(result));
          } else if (request.url === '/sibling') {
            response.setHeader('Content-Type', 'text/html');
            response.end('<!doctype html><title>Unrelated sibling frame</title>');
          } else if (request.url === '/') {
            response.setHeader('Content-Type', 'text/html');
            response.end(
              '<!doctype html><html><head><title>MCP Apps protocol harness</title><style>body{margin:24px;background:#e9edf4;font:14px system-ui}iframe{display:block;width:100%;height:680px;border:1px solid #bac6d8;border-radius:10px;background:white}</style></head><body><script src="/host.js"></script></body></html>',
            );
          } else {
            response.writeHead(404).end();
          }
        } catch (error) {
          response.writeHead(500, { 'Content-Type': 'text/plain' }).end(String(error));
        }
      });
      server.listen(0, '127.0.0.1');
      await once(server, 'listening');
      try {
        await use({
          url: `http://127.0.0.1:${server.address().port}`,
          realServer: !!client,
          resource,
        });
      } finally {
        server.closeAllConnections();
        await new Promise((resolve) => server.close(resolve));
        await client?.close();
      }
    },
    { scope: 'worker', timeout: 60_000 },
  ],
  viewer: async ({ page, harnessServer }, use) => {
    await page.goto(harnessServer.url);
    await use(async (capabilities, permissions = {}) => {
      await page.evaluate(
        ([caps, permissions]) => window.harness.mount(caps, permissions),
        [capabilities, permissions],
      );
      const frame = page.frameLocator('#viewer');
      await expect(frame.locator('#connection')).toHaveText('Connected');
      return frame;
    });
  },
});

export { expect };

export const svg = (
  body = '<rect width="200" height="100" fill="#245bdd"/>',
  width = 200,
  height = 100,
) => `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}">${body}</svg>`;

export const svgResult = (text = svg()) => ({
  content: [
    {
      type: 'resource',
      resource: {
        uri: 'd2://render/diagram.svg',
        mimeType: 'image/svg+xml',
        text,
      },
    },
  ],
});

export const textResult = (text) => ({ content: [{ type: 'text', text }] });

export async function sendInput(page, args = { code: 'a -> b' }) {
  await page.evaluate((args) => window.harness.input(args), args);
}

export async function sendResult(page, result) {
  await page.evaluate((result) => window.harness.result(result), result);
}
