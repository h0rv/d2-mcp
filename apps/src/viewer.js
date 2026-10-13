import { App } from '@modelcontextprotocol/ext-apps';

// All host content is untrusted. Never insert result text or SVG into live HTML.
// SVG is displayed as an inert image and exports are re-encoded as PNG.
const MAX_SOURCE = 256 * 1024;
const MAX_SVG = 2 * 1024 * 1024;
const MAX_IMAGE = 16 * 1024 * 1024;
const MAX_PIXELS = 16 * 1024 * 1024;
const encoder = new TextEncoder();

const $ = (id) => document.getElementById(id);

const app = new App({ name: 'd2-viewer', version: '0.1.0' }, {}, { autoResize: true });
let canDownload = false,
  disposed = false;
let sequence = 0,
  previewURL;
let scale = 1,
  panX = 0,
  panY = 0,
  drag;

function status(message, error = false) {
  $('status').textContent = String(message).slice(0, 4096);
  $('status').dataset.error = String(error);
}

function buttons() {
  $('copy-source').disabled = disposed || !$('source').value;
  $('download-image').disabled = disposed || !canDownload || !previewURL;
  for (const id of ['zoom-in', 'zoom-out', 'fit']) $(id).disabled = disposed || !previewURL;
}

function clearPreview() {
  previewURL = undefined;
  $('diagram').removeAttribute('src');
  $('diagram').hidden = true;
  $('ascii').hidden = true;
  $('ascii').textContent = '';
  $('empty').hidden = false;
  buttons();
}

function invalidate() {
  sequence++;
  drag = undefined;
  buttons();
  return sequence;
}

function dimensions(width, height) {
  if (
    !Number.isFinite(width) ||
    !Number.isFinite(height) ||
    width <= 0 ||
    height <= 0 ||
    width > 8192 ||
    height > 8192 ||
    width * height > MAX_PIXELS
  ) {
    throw new Error('Preview dimensions exceed the browser limit (8192 per side, 16 megapixels).');
  }
}

function svgBlob(text) {
  if (typeof text !== 'string' || text.length > MAX_SVG || encoder.encode(text).length > MAX_SVG) {
    throw new Error('SVG preview exceeds the 2 MiB browser limit. Try PNG or a smaller diagram.');
  }
  if (/<!DOCTYPE|<!ENTITY/i.test(text)) throw new Error('SVG declarations are not supported.');
  const xml = new DOMParser().parseFromString(text, 'image/svg+xml');
  const root = xml.documentElement;
  if (
    root.localName !== 'svg' ||
    root.namespaceURI !== 'http://www.w3.org/2000/svg' ||
    xml.querySelector('parsererror')
  ) {
    throw new Error('The server returned invalid SVG.');
  }
  // D2 may supply only a viewBox. Normalize a bounded intrinsic size before
  // decoding; never let browser fallback sizing hide enormous dimensions.
  const pixels = (value) =>
    /^(?:\d+(?:\.\d+)?|\.\d+)(?:px)?$/.test(value || '') ? Number(value.replace(/px$/, '')) : NaN;

  const box = root
    .getAttribute('viewBox')
    ?.trim()
    .split(/[\s,]+/)
    .map(Number);
  const validBox = box?.length === 4 && box.every(Number.isFinite);
  const width = root.hasAttribute('width')
    ? pixels(root.getAttribute('width'))
    : validBox
      ? box[2]
      : NaN;
  const height = root.hasAttribute('height')
    ? pixels(root.getAttribute('height'))
    : validBox
      ? box[3]
      : NaN;
  dimensions(width, height);
  root.setAttribute('width', String(width));
  root.setAttribute('height', String(height));
  return new Blob([new XMLSerializer().serializeToString(root)], { type: 'image/svg+xml' });
}

function pngBlob(data) {
  if (
    typeof data !== 'string' ||
    data.length > Math.ceil(MAX_IMAGE / 3) * 4 ||
    !/^[A-Za-z0-9+/]*={0,2}$/.test(data)
  ) {
    throw new Error('PNG preview is invalid or exceeds the 16 MiB browser limit.');
  }
  const bytes = Uint8Array.from(atob(data), (ch) => ch.charCodeAt(0));
  const header = [137, 80, 78, 71, 13, 10, 26, 10, 0, 0, 0, 13, 73, 72, 68, 82];
  if (bytes.length < 33 || !header.every((value, i) => bytes[i] === value))
    throw new Error('The server returned invalid PNG.');
  const view = new DataView(bytes.buffer);
  dimensions(view.getUint32(16), view.getUint32(20));
  return new Blob([bytes], { type: 'image/png' });
}

function imageURL(blob) {
  // The standard host CSP permits data: images, but need not permit blob:.
  // A second meta policy cannot relax that outer policy in an opaque iframe.
  return new Promise((resolve, reject) => {
    const reader = new FileReader();

    reader.onload = () => resolve(reader.result);

    reader.onerror = () => reject(new Error('The image could not be read.'));

    reader.readAsDataURL(blob);
  });
}

function transform() {
  $('diagram').style.transform =
    `translate(calc(-50% + ${panX}px), calc(-50% + ${panY}px)) scale(${scale})`;
}

function fit() {
  const image = $('diagram'),
    box = $('viewport');
  if (!previewURL) return;
  scale = Math.min(
    (box.clientWidth - 40) / image.naturalWidth,
    (box.clientHeight - 60) / image.naturalHeight,
    1,
  );
  panX = panY = 0;
  transform();
}

async function showResult(result, id) {
  if (id !== sequence) return;
  clearPreview();
  try {
    if (!result || !Array.isArray(result.content) || result.content.length > 16)
      throw new Error('The host returned an unsupported tool result.');
    if (result.isError) {
      const detail = result.structuredContent?.error;
      throw new Error(
        typeof detail?.message === 'string'
          ? detail.message
          : result.content.find((c) => c.type === 'text')?.text || 'Rendering failed.',
      );
    }
    const artifact = result.content.find(
      (c) =>
        (c.type === 'image' && c.mimeType === 'image/png') ||
        (c.type === 'resource' && c.resource?.mimeType === 'image/svg+xml'),
    );
    if (!artifact) {
      const text = result.content.find((c) => c.type === 'text')?.text;
      if (typeof text === 'string' && text.length <= MAX_SVG) {
        $('ascii').textContent = text;
        $('ascii').hidden = false;
        $('empty').hidden = true;
        status('Text result received.');
        return;
      }
      throw new Error(
        'No inline preview is available. Server file links stay in the ordinary tool result.',
      );
    }
    const blob =
      artifact.type === 'image' ? pngBlob(artifact.data) : svgBlob(artifact.resource.text);
    const url = await imageURL(blob);
    if (id !== sequence) return;
    const image = new Image();
    image.src = url;
    await image.decode();
    if (id !== sequence) return;
    dimensions(image.naturalWidth, image.naturalHeight);
    previewURL = url;
    $('diagram').src = url;
    await $('diagram').decode();
    if (id !== sequence) return;
    $('diagram').hidden = false;
    $('empty').hidden = true;
    fit();
    status('Preview ready. Drag to pan; use the zoom controls.');
  } catch (error) {
    if (id === sequence) {
      clearPreview();
      status(error.message, true);
    }
  } finally {
    if (id === sequence) buttons();
  }
}

app.ontoolinput = ({ arguments: args }) => {
  if (disposed) return;
  invalidate();
  clearPreview();
  $('source').value = '';
  $('source-panel').open = false;
  if (args && typeof args === 'object') {
    if (
      typeof args.code === 'string' &&
      args.code.length <= MAX_SOURCE &&
      encoder.encode(args.code).length <= MAX_SOURCE
    )
      $('source').value = args.code;
  }
  $('source-panel').hidden = !$('source').value;
  status(
    $('source').value
      ? 'Waiting for the diagram.'
      : 'Source is not included. File and document inputs are not fetched.',
  );
  buttons();
};

app.ontoolresult = (result) => {
  if (!disposed) {
    const id = invalidate();
    void showResult(result, id);
  }
};

app.ontoolcancelled = () => {
  if (!disposed) {
    invalidate();
    clearPreview();
    status('Rendering cancelled.');
  }
};

app.onhostcontextchanged = (context) => {
  if (!disposed && ['light', 'dark'].includes(context.theme))
    document.documentElement.dataset.theme = context.theme;
};

function dispose() {
  disposed = true;
  invalidate();
  clearPreview();
  $('source').value = '';
  $('source-panel').hidden = true;
  $('connection').textContent = 'Closed';
  status('Viewer closed.');
  buttons();
}

app.onteardown = async () => {
  dispose();
  return {};
};

$('copy-source').addEventListener('click', async () => {
  const text = $('source').value,
    id = sequence;
  if (disposed || !text) return;
  try {
    await navigator.clipboard.writeText(text);
    if (id === sequence) status('D2 source copied.');
  } catch {
    if (id !== sequence) return;
    $('source-panel').open = true;
    $('source').focus();
    $('source').select();
    status('Source selected. Press Ctrl+C or ⌘C to copy.');
  }
});

for (const [id, factor] of [
  ['zoom-in', 1.25],
  ['zoom-out', 0.8],
])
  $(id).addEventListener('click', () => {
    scale = Math.min(8, Math.max(0.05, scale * factor));
    transform();
  });

$('fit').addEventListener('click', fit);

$('viewport').addEventListener('pointerdown', (event) => {
  if (!previewURL || event.button !== 0) return;
  drag = { x: event.clientX - panX, y: event.clientY - panY };
  $('viewport').setPointerCapture(event.pointerId);
});

$('viewport').addEventListener('pointermove', (event) => {
  if (drag) {
    panX = event.clientX - drag.x;
    panY = event.clientY - drag.y;
    transform();
  }
});

for (const type of ['pointerup', 'pointercancel', 'lostpointercapture'])
  $('viewport').addEventListener(type, () => {
    drag = undefined;
  });

new ResizeObserver(fit).observe($('viewport'));

async function download(resource) {
  const id = sequence;
  try {
    const result = await app.downloadFile({ contents: [{ type: 'resource', resource }] });
    if (id === sequence)
      status(
        result.isError
          ? 'The host declined or cancelled the download.'
          : 'Download handed to the host.',
        !!result.isError,
      );
  } catch (error) {
    if (id === sequence) status(error.message || 'Download failed.', true);
  }
}

$('download-image').addEventListener('click', () => {
  if (disposed || !canDownload || !previewURL) return;
  try {
    const image = $('diagram');
    dimensions(image.naturalWidth, image.naturalHeight);
    const canvas = document.createElement('canvas');
    canvas.width = image.naturalWidth;
    canvas.height = image.naturalHeight;
    canvas.getContext('2d').drawImage(image, 0, 0);
    const blob = canvas.toDataURL('image/png').split(',')[1];
    if (!blob || blob.length > Math.ceil(MAX_IMAGE / 3) * 4)
      throw new Error('PNG export exceeds the 16 MiB browser limit.');
    void download({ uri: 'file:///diagram.png', mimeType: 'image/png', blob });
  } catch (error) {
    status(error.message || 'PNG export failed.', true);
  }
});

window.addEventListener('pagehide', () => {
  dispose();
  void app.close();
});

(async () => {
  try {
    await app.connect();
    if (disposed) return;
    const capabilities = app.getHostCapabilities();
    canDownload = !!capabilities?.downloadFile;
    $('connection').textContent = 'Connected';
    const theme = app.getHostContext()?.theme;
    if (['light', 'dark'].includes(theme)) document.documentElement.dataset.theme = theme;
    $('download-image').hidden = !canDownload;
    if ($('status').textContent === 'Waiting for the host.') status('Waiting for a diagram.');
    buttons();
  } catch (error) {
    if (disposed) return;
    $('connection').textContent = 'Host unavailable';
    status(`Could not connect to an MCP Apps host: ${error.message}`, true);
  }
})();
