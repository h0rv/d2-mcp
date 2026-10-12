import assert from 'node:assert/strict';
import {test} from 'node:test';
import {Miniflare, convertV4MiniflareOptions} from 'miniflare';

const options = token => convertV4MiniflareOptions({
  name: 'd2-mcp-test',
  modules: [{type:'ESModule',path:'worker.js'}, {type:'ESModule',path:'build/wasm_exec.js'}, {type:'CompiledWasm',path:'build/d2.wasm'}],
  compatibilityDate: '2026-10-12',
  compatibilityFlags: ['enable_request_signal','request_signal_passthrough'],
  durableObjects: {RENDERER: {className:'Renderer',useSQLite:true}},
  bindings: token ? {D2_MCP_TOKEN: token} : {},
});
const token = 'local-test-fixture';
const headers = {Authorization: `Bearer ${token}`, 'Content-Type':'application/json', Accept:'application/json, text/event-stream', 'MCP-Protocol-Version':'2025-11-25'};

test('authenticated Go MCP: discovery, layouts, formats, errors and bounds', async () => {
  const mf = new Miniflare(options(token));
  let id = 0;
  async function rpc(method, params, version = '2025-11-25') {
    const current = version === '2026-07-28';
    if (current) params = {...params, _meta: {'io.modelcontextprotocol/protocolVersion': version, 'io.modelcontextprotocol/clientCapabilities': {}}};
    const response = await mf.dispatchFetch('https://d2.test/mcp', {method:'POST',headers:{...headers, 'MCP-Protocol-Version':version, 'Mcp-Method':method, ...(params?.name ? {'Mcp-Name':params.name} : {})},body:JSON.stringify({jsonrpc:'2.0',id:++id,method,params})});
    assert.equal(response.status,200);
    assert.equal(response.headers.get('mcp-session-id'),null);
    const message = await response.json();
    assert.equal(message.error,undefined,JSON.stringify(message));
    return message.result;
  }
  try {
    assert.equal((await mf.dispatchFetch('https://d2.test/mcp')).status,401);
    assert.equal((await mf.dispatchFetch('https://d2.test/other',{headers})).status,404);
    assert.equal((await mf.dispatchFetch('https://d2.test/mcp',{headers})).status,405);
    const init = await rpc('initialize',{protocolVersion:'2025-11-25',capabilities:{},clientInfo:{name:'integration-test',version:'1'}});
    assert.equal(init.serverInfo.name,'d2-mcp');
    const list = await rpc('tools/list',{},'2026-07-28');
    assert.equal(list.tools.length,3);
    for (const [layout,format] of [['dagre','svg'],['elk','png'],['tala','ascii']]) {
      const result = await rpc('tools/call',{name:'render-d2',arguments:{code:'client -> api -> database',layout,format}});
      assert.equal(result.isError,undefined,JSON.stringify(result));
      assert.equal(result.structuredContent.options.layout,layout);
      assert.ok(result.structuredContent.artifact.bytes>20);
      if(format==='png') assert.equal(Buffer.from(result.content.find(c=>c.type==='image').data,'base64').subarray(0,8).toString('hex'),'89504e470d0a1a0a');
      if(format==='svg') assert.match(result.content.find(c=>c.type==='resource').resource.text,/<svg/);
    }
    for(const args of [{code:'a: {'},{file_path:'secret.d2'},{code:'a.icon: https://example.com/image.png'}]) {
      const result=await rpc('tools/call',{name:'render-d2',arguments:args});
      assert.equal(result.isError,true);
    }
    assert.equal((await mf.dispatchFetch('https://d2.test/mcp',{method:'POST',headers,body:'[]'})).status,400);
    assert.equal((await mf.dispatchFetch('https://d2.test/mcp',{method:'POST',headers,body:' '.repeat(2*1024*1024+1)})).status,413);
    assert.equal((await mf.dispatchFetch('https://d2.test/mcp',{method:'POST',headers:{...headers,Origin:'https://other.test'},body:'{}'})).status,403);
    assert.equal((await rpc('tools/call',{name:'compile-d2',arguments:{code:'a -> b'}},'2026-07-28')).structuredContent.valid,true);
  } finally { await mf.dispose(); }
});

test('missing credential fails closed', async () => {
  const mf=new Miniflare(options());
  try { assert.equal((await mf.dispatchFetch('https://d2.test/mcp',{headers})).status,503); }
  finally { await mf.dispose(); }
});

test('cancelled current-protocol requests produce no result and leave the bridge usable', async () => {
  // The test entry supplies an already-aborted signal, without a timing race.
  const mf=new Miniflare(convertV4MiniflareOptions({
    name:'cancelled-bridge', compatibilityDate:'2026-10-12',
    compatibilityFlags:['enable_request_signal','request_signal_passthrough'],
    modules:[{type:'ESModule',path:'cancellation-test-worker.js'}, {type:'ESModule',path:'worker.js'}, {type:'ESModule',path:'build/wasm_exec.js'}, {type:'CompiledWasm',path:'build/d2.wasm'}],
  }));
  const params = {
    name: 'compile-d2', arguments: {code: 'a -> b'},
    _meta: {'io.modelcontextprotocol/protocolVersion':'2026-07-28', 'io.modelcontextprotocol/clientCapabilities':{}},
  };
  const request = {
    method: 'POST',
    headers: {...headers, 'MCP-Protocol-Version':'2026-07-28', 'Mcp-Method':'tools/call', 'Mcp-Name':'compile-d2'},
    body: JSON.stringify({jsonrpc:'2.0', id:1, method:'tools/call', params}),
  };
  try {
    const cancelled=await mf.dispatchFetch('https://d2.test/mcp',{...request,headers:{...request.headers,'X-Test-Abort':'1'}});
    assert.equal(await cancelled.text(),''); // SDK closes a cancelled transport without a result.
    const next=await mf.dispatchFetch('https://d2.test/mcp',request);
    assert.equal((await next.json()).result.structuredContent.valid,true);
  } finally { await mf.dispose(); }
});
