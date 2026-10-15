// Test-only entry point: pass a cancelled request through the actual Wasm bridge.
import { Renderer } from './worker.js';

export default {
  fetch(request) {
    const init = request.headers.has('X-Test-Abort') ? { signal: AbortSignal.abort() } : {};
    return new Renderer().fetch(new Request(request, init));
  },
};
