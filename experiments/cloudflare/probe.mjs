import {Miniflare, convertV4MiniflareOptions} from 'miniflare';
import {writeFile, mkdir} from 'node:fs/promises';
import {performance} from 'node:perf_hooks';

await mkdir('results', {recursive:true});
const mode=process.argv[2]??'worker';
if (!['worker','do'].includes(mode)) throw new Error('Expected worker or do');
const start = performance.now();
const mf = new Miniflare(convertV4MiniflareOptions({
  name: 'd2-local-probe', modules: [{type:'ESModule',path:'worker.js'},{type:'ESModule',path:'build/wasm_exec.js'},{type:'CompiledWasm',path:'build/d2.wasm'}],
  compatibilityDate: '2026-10-10',
  compatibilityFlags: ['no_nodejs_compat', 'no_nodejs_compat_v2'],
  durableObjects: {RENDERER: {className:'Renderer',useSQLite:true}},
  inspectorPort: 0,
}));
const results = {node:process.version,localOnly:true,mode,cases:[]};
let socket;
let sequence = 0;
const pending = new Map();
async function inspect(method, params={}) {
  const id = ++sequence;
  return new Promise((resolve,reject)=>{
    pending.set(id,{resolve,reject});
    socket.send(JSON.stringify({id,method,params}));
  });
}
function summarize(profile) {
  const names = new Map(profile.nodes.map(node=>[node.id,node.callFrame.functionName]));
  const time = {activeMs:0,idleMs:0,gcMs:0,samples:profile.samples.length};
  for (let i=0;i<profile.samples.length;i++) {
    const name = names.get(profile.samples[i]);
    const ms = profile.timeDeltas[i]/1000;
    if (name==='(idle)') time.idleMs+=ms;
    else {time.activeMs+=ms;if(name==='(garbage collector)')time.gcMs+=ms;}
  }
  return time;
}
try {
  await mf.ready;
  results.readyWallMs = performance.now()-start;
  const inspectorURL = new URL(await mf.getInspectorURL());
  inspectorURL.protocol='http:'; inspectorURL.pathname='/json/list';
  const targets=await (await fetch(inspectorURL)).json();
  const target=targets.find(t=>t.title.includes('d2-local-probe'))??targets[0];
  socket=new WebSocket(target.webSocketDebuggerUrl);
  await new Promise((resolve,reject)=>{socket.onopen=resolve;socket.onerror=reject;});
  socket.onmessage=event=>{
    const message=JSON.parse(event.data);const callback=pending.get(message.id);
    if(callback){pending.delete(message.id);message.error?callback.reject(message.error):callback.resolve(message.result);}
  };
  await inspect('Profiler.enable');
  await inspect('Profiler.setSamplingInterval',{interval:1000});
  results.initialHeap=await inspect('Runtime.getHeapUsage');
  console.log(JSON.stringify({readyWallMs:results.readyWallMs,initialHeap:results.initialHeap}));
  const small = 'client -> api -> database\ndatabase.shape: cylinder';
  const cases = [
    {name:'dagre-cold',code:small,layout:'dagre'},
    {name:'dagre-warm',code:small,layout:'dagre'},
    {name:'elk-warm',code:small,layout:'elk'},
    {name:'tala-warm',code:small,layout:'tala'},
    {name:'durable-object',path:'/do',code:small,layout:'dagre'},
    {name:'invalid-source',code:'a: {',layout:'dagre',expectedStatus:422},
    {name:'25-nodes',code:Array.from({length:24},(_,i)=>`n${i} -> n${i+1}`).join('\n'),layout:'dagre'},
    {name:'100-nodes-grid',code:'grid-columns: 10\n'+Array.from({length:100},(_,i)=>`n${i}`).join('\n'),layout:'dagre'},
    {name:'native-png',code:small,layout:'dagre',format:'png'},
    {name:'durable-object-warm',path:'/do',code:small,layout:'dagre'},
    ...Array.from({length:3},(_,i)=>({name:`dagre-repeat-${i+1}`,code:small,layout:'dagre'})),
  ];
  for (const test of cases) {
    await inspect('Profiler.start');
    const before = performance.now();
    const response = await mf.dispatchFetch(`http://local.test${mode==='do'?'/do':test.path??'/'}`, {method:'POST',body:JSON.stringify(test)});
    const data = await response.json();
    const result = {name:test.name,status:response.status,wallMs:performance.now()-before,...data};
    const {profile}=await inspect('Profiler.stop');
    result.localSampledCPU=summarize(profile);
    result.inspectorHeap=await inspect('Runtime.getHeapUsage');
    if (data.svg) {
      if (!data.svg.includes('<svg')) throw new Error('Missing SVG');
      if (test.name==='dagre-cold') await writeFile(`results/${mode}-example.svg`,data.svg);
      delete result.svg;
    }
    results.cases.push(result);
    console.log(JSON.stringify(result));
    if (response.status !== (test.expectedStatus??200)) throw new Error(`Unexpected response for ${test.name}`);
    if (result.runtime.wasmMemoryBytes >= 128*1024*1024) {
      results.stopped = 'Wasm memory alone meets/exceeds deployed 128 MiB isolate limit';
      break;
    }
  }
} catch (error) {
  results.error = String(error.stack??error);
  console.error(error);
  process.exitCode = 1;
} finally {
  await writeFile(`results/${mode}.json`,JSON.stringify(results,null,2)+'\n');
  socket?.close();
  await mf.dispose();
}
