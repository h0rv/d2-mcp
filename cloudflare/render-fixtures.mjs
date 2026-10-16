// Run after npm run build. Preserve real MCP artifacts as integration fixtures.
// These files are not bundled into or served by the Worker.
import { readFile, writeFile } from 'node:fs/promises';
import './build/wasm_exec.js';

const go = new Go();
const { instance } = await WebAssembly.instantiate(await readFile('build/d2.wasm'), go.importObject);
void go.run(instance);
const code = (await readFile('testdata/example.d2', 'utf8')).trimEnd();
for (const format of ['svg', 'png', 'ascii']) {
  const response = JSON.parse(await d2MCP(JSON.stringify({
    method: 'POST', url: 'https://example.test/mcp',
    headers: {
      'Content-Type': 'application/json', Accept: 'application/json, text/event-stream',
      'MCP-Protocol-Version': '2025-11-25',
    },
    body: JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'tools/call', params: {
      name: 'render-d2', arguments: { code, format, padding: 24 },
    } }),
  }), new AbortController().signal));
  const result = JSON.parse(response.body).result;
  if (response.status !== 200 || result?.isError) throw new Error(JSON.stringify(response));
  const data = format === 'png'
    ? Buffer.from(result.content.find((item) => item.type === 'image').data, 'base64')
    : format === 'svg'
      ? result.content.find((item) => item.type === 'resource').resource.text
      : result.content.find((item) => item.type === 'text').text;
  await writeFile(`testdata/example.${format === 'ascii' ? 'txt' : format}`, data);
}
