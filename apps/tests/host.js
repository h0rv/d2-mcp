// A protocol harness, not a claim of compatibility testing in a product host.
import {
  AppBridge,
  PostMessageTransport,
  buildAllowAttribute,
} from '@modelcontextprotocol/ext-apps/app-bridge';

const calls = [],
  downloads = [],
  messages = [],
  errors = [];
let bridge, frame;

window.harness = {
  calls,
  downloads,
  messages,
  errors,
  downloadResult: {},

  async mount(capabilities = { downloadFile: {} }, permissions = {}) {
    frame = document.createElement('iframe');
    frame.id = 'viewer';
    frame.title = 'D2 MCP App';
    // No same-origin, popups, navigation, forms or browser download privileges.
    frame.setAttribute('sandbox', 'allow-scripts');
    // A sandbox without allow-same-origin has an opaque origin. 'src' (the
    // default allowlist) cannot match it; the host must delegate requested
    // features to this frame explicitly. This adds no sandbox privileges.
    frame.setAttribute(
      'allow',
      buildAllowAttribute(permissions)
        .split(';')
        .filter(Boolean)
        .map((feature) => `${feature.trim()} *`)
        .join('; '),
    );
    document.body.append(frame);
    window.addEventListener('message', (event) => {
      if (event.source === frame.contentWindow) messages.push(event.data);
    });
    bridge = new AppBridge(null, { name: 'd2-browser-harness', version: '1.0.0' }, capabilities, {
      hostContext: { theme: 'light', displayMode: 'inline' },
    });

    bridge.onerror = (error) => errors.push(String(error));

    bridge.oncalltool = async (params) => {
      calls.push(params);
      throw new Error('A viewer must never call server tools');
    };

    bridge.ondownloadfile = async (params) => {
      downloads.push(params);
      if (window.harness.deferDownload)
        return new Promise((resolve) => {
          window.harness.finishDownload = resolve;
        });
      if (window.harness.downloadError) throw new Error(window.harness.downloadError);
      return window.harness.downloadResult;
    };

    const initialized = new Promise((resolve) => {
      bridge.oninitialized = resolve;
    });
    await bridge.connect(new PostMessageTransport(frame.contentWindow, frame.contentWindow));
    frame.src = '/viewer.html';
    await initialized;
  },

  input(args) {
    return bridge.sendToolInput({ arguments: args });
  },

  result(result) {
    return bridge.sendToolResult(result);
  },

  async render(args) {
    await this.input(args);
    const response = await fetch('/tools/call', {
      method: 'POST',
      body: JSON.stringify({ name: 'render-d2', arguments: args }),
    });
    if (!response.ok) throw new Error(await response.text());
    const result = await response.json();
    await this.result(result);
    return result;
  },

  cancel() {
    return bridge.sendToolCancelled({ reason: 'Cancelled by the host' });
  },

  raw(message) {
    frame.contentWindow.postMessage(message, '*');
  },

  teardown() {
    return bridge.teardownResource({});
  },
};
