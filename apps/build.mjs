import { build } from 'esbuild';
import { createHash } from 'node:crypto';
import { mkdir, readFile, writeFile } from 'node:fs/promises';

const output = await build({ entryPoints: ['src/viewer.js'], bundle: true, write: false,
  format: 'iife', platform: 'browser', target: 'es2022', minify: true, legalComments: 'inline', metafile: true });
const script = output.outputFiles[0].text.replaceAll('</script', '<\\/script');
const hash = createHash('sha256').update(script).digest('base64');
const template = await readFile('src/viewer.html', 'utf8');
// Carry dependency licenses inside the embedded resource too, so a Go-only
// distribution retains notices for the JavaScript it serves.
const packages = new Set(Object.entries(Object.values(output.metafile.outputs)[0].inputs)
  .filter(([path, value]) => path.startsWith('node_modules/') && value.bytesInOutput > 0)
  .map(([path]) => path.split('/').slice(1, path.startsWith('node_modules/@') ? 3 : 2).join('/')));
const notices = ['D2 MCP viewer\n' + await readFile('../LICENSE', 'utf8')];
for (const name of [...packages].sort()) {
  const info = JSON.parse(await readFile(`node_modules/${name}/package.json`, 'utf8'));
  notices.push(`${name} ${info.version}\n${await readFile(`node_modules/${name}/LICENSE`, 'utf8')}`);
}
await mkdir('dist', { recursive: true });
await writeFile('dist/viewer.html', template.replace('BUNDLE_HASH', hash).replace('/* BUNDLE */', () => script) +
  '\n<!-- Bundled license notices\n' + notices.join('\n\n').replaceAll('-->', '-- >').replace(/[ \t]+$/gm, '') + '\n-->\n');
