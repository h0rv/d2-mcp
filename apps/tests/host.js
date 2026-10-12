// A protocol harness, not a claim of compatibility testing in a product host.
import { AppBridge, PostMessageTransport } from '@modelcontextprotocol/ext-apps/app-bridge';

const calls = [], downloads = [], messages = [], errors = [];
let bridge, frame;
window.harness = {
  calls, downloads, messages, errors,
  downloadResult: {},
  async mount(capabilities = { serverTools: {}, downloadFile: {} }, realServer = false) {
    frame = document.createElement('iframe');
    frame.id = 'viewer';
    frame.title = 'D2 MCP App';
    // No same-origin, popups, navigation, forms or browser download privileges.
    frame.setAttribute('sandbox', 'allow-scripts');
    document.body.append(frame);
    window.addEventListener('message', (event) => {
      if (event.source === frame.contentWindow) messages.push(event.data);
    });
    bridge = new AppBridge(null, { name: 'd2-browser-harness', version: '1.0.0' }, capabilities,
      { hostContext: { theme: 'light', displayMode: 'inline' } });
    bridge.onerror = (error) => errors.push(String(error));
    bridge.oncalltool = async (params, extra) => {
      const call = { params, cancelled: false };
      calls.push(call);
      const signal = extra.mcpReq?.signal || extra.signal;
      signal?.addEventListener('abort', () => { call.cancelled = true; });
      if (realServer) {
        const response = await fetch('/tools/call', { method: 'POST', body: JSON.stringify(params) });
        if (!response.ok) throw new Error(await response.text());
        return response.json();
      }
      return new Promise((resolve, reject) => { call.resolve = resolve; call.reject = reject; });
    };
    bridge.ondownloadfile = async (params) => {
      downloads.push(params);
      if (window.harness.downloadError) throw new Error(window.harness.downloadError);
      return window.harness.downloadResult;
    };
    const initialized = new Promise((resolve) => { bridge.oninitialized = resolve; });
    await bridge.connect(new PostMessageTransport(frame.contentWindow, frame.contentWindow));
    frame.src = '/viewer.html';
    await initialized;
  },
  input(args) { return bridge.sendToolInput({ arguments: args }); },
  result(result) { return bridge.sendToolResult(result); },
  cancel() { return bridge.sendToolCancelled({ reason: 'Cancelled by the host' }); },
  resolve(index, result) { calls[index].resolve(result); },
  reject(index, message) { calls[index].reject(new Error(message)); },
  raw(message) { frame.contentWindow.postMessage(message, '*'); },
  teardown() { return bridge.teardownResource({}); },
};
