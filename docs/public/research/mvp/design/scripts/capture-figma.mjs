#!/usr/bin/env node
// capture-figma.mjs — produce GENUINE OD Figma capture IR for each design page,
// headless, with zero fabrication.
//
// It injects the OpenDesign clipper's OWN page-capture runtime
// (open-design/clipper/capture.js → window.__odCapture) into each live page over
// CDP and reads back `figmaIr` — the exact intermediate representation the daemon's
// `od library figma <id>` endpoint serves for clipper-captured assets, and the
// exact input the "OD Figma Import" Figma plugin (figma-plugin/code.js) consumes.
// The pages are self-contained (inline CSS/SVG, data-URI images), so no cross-origin
// worker inlining is needed and the IR is complete as captured.
//
// Output:
//   figma/captures/<area>-<screen>.od-figma.json   one genuine IR per page
//   figma/thready.od-figma.json                    combined super-root of all pages
//   figma/capture-manifest.json                    provenance + validation report
//
// Usage: node capture-figma.mjs <capture.js path> <out figma dir> <jobs.json>
//   jobs.json = [ { url, name, width, mobile } ]

import { spawn } from 'node:child_process';
import { mkdirSync, writeFileSync, readFileSync } from 'node:fs';
import { setTimeout as sleep } from 'node:timers/promises';

const [captureJsPath, figmaDir, jobsPath] = process.argv.slice(2);
const CAPTURE_JS = readFileSync(captureJsPath, 'utf8');
const jobs = JSON.parse(readFileSync(jobsPath, 'utf8'));
const CHROMIUM = process.env.CHROMIUM || '/usr/bin/chromium';
const PORT = Number(process.env.CDP_PORT || 9371);

mkdirSync(`${figmaDir}/captures`, { recursive: true });

const userDataDir = `/tmp/capture-figma-${process.pid}`;
const chrome = spawn(CHROMIUM, [
  '--headless=new', `--remote-debugging-port=${PORT}`, `--user-data-dir=${userDataDir}`,
  '--no-sandbox', '--disable-gpu', '--hide-scrollbars', '--no-first-run',
  '--force-color-profile=srgb', '--allow-file-access-from-files', 'about:blank',
], { stdio: ['ignore', 'ignore', 'pipe'] });
let stderr = ''; chrome.stderr.on('data', d => stderr += d);

async function wsUrl() {
  for (let i = 0; i < 100; i++) {
    try { const r = await fetch(`http://127.0.0.1:${PORT}/json/version`); if (r.ok) return (await r.json()).webSocketDebuggerUrl; } catch {}
    await sleep(100);
  }
  throw new Error('no devtools endpoint\n' + stderr);
}
function connect(url) {
  const ws = new WebSocket(url); const pending = new Map(); const listeners = []; let id = 0;
  const ready = new Promise((res, rej) => { ws.onopen = () => res(); ws.onerror = rej; });
  ws.onmessage = ev => { const m = JSON.parse(ev.data);
    if (m.id && pending.has(m.id)) { const { resolve, reject } = pending.get(m.id); pending.delete(m.id); m.error ? reject(new Error(JSON.stringify(m.error))) : resolve(m.result); }
    else listeners.forEach(l => l(m)); };
  const send = (method, params = {}, sessionId) => new Promise((resolve, reject) => { const mid = ++id; pending.set(mid, { resolve, reject }); ws.send(JSON.stringify({ id: mid, method, params, ...(sessionId ? { sessionId } : {}) })); });
  return { ws, ready, send, onEvent: fn => listeners.push(fn) };
}

const cli = connect(await wsUrl());
await cli.ready;

function validateIR(ir) {
  const errs = [];
  if (ir?.version !== 1) errs.push('version!=1');
  if (!ir?.source?.url) errs.push('missing source.url');
  if (!ir?.source?.viewport) errs.push('missing source.viewport');
  if (!Array.isArray(ir?.fonts)) errs.push('fonts not array');
  if (!ir?.root || ir.root.type !== 'FRAME') errs.push('root not FRAME');
  let n = 0; (function walk(x) { if (!x || typeof x !== 'object') return; n++; (x.children || []).forEach(walk); })(ir?.root);
  if (n < 2) errs.push('root has <2 nodes');
  return { ok: errs.length === 0, errs, nodeCount: n };
}

async function capture(job) {
  const { targetId } = await cli.send('Target.createTarget', { url: 'about:blank' });
  const { sessionId } = await cli.send('Target.attachToTarget', { targetId, flatten: true });
  const S = (m, p) => cli.send(m, p, sessionId);
  let loaded = false; cli.onEvent(m => { if (m.sessionId === sessionId && m.method === 'Page.loadEventFired') loaded = true; });
  await S('Page.enable'); await S('Runtime.enable');
  await S('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-color-scheme', value: 'light' }] });
  await S('Page.addScriptToEvaluateOnNewDocument', { source: `try{localStorage.setItem('thready-theme','light')}catch(e){}` });
  await S('Emulation.setDeviceMetricsOverride', { width: job.width || 1440, height: 900, deviceScaleFactor: 2, mobile: !!job.mobile, screenWidth: job.width || 1440, screenHeight: 900 });
  loaded = false;
  await S('Page.navigate', { url: job.url });
  for (let i = 0; i < 200 && !loaded; i++) await sleep(50);
  await sleep(800);
  // inject the clipper's capture runtime, then run it
  await S('Runtime.evaluate', { expression: CAPTURE_JS });
  const res = await S('Runtime.evaluate', {
    expression: `JSON.stringify((window.__odCapture({includeImages:true})||{}).figmaIr||null)`,
    returnByValue: true,
  });
  await cli.send('Target.closeTarget', { targetId });
  const raw = res.result?.value;
  if (!raw || raw === 'null') throw new Error('__odCapture returned no figmaIr');
  return JSON.parse(raw);
}

// deep-translate every node's absolute x by dx (for combined-board layout)
function translateX(node, dx) {
  if (!node || typeof node !== 'object') return;
  if (typeof node.x === 'number') node.x += dx;
  (node.children || []).forEach(c => translateX(c, dx));
}

const report = [];
const boardChildren = [];
let cursorX = 0; const GAP = 120;
let fontsUnion = new Map();

for (const job of jobs) {
  try {
    const ir = await capture(job);
    const v = validateIR(ir);
    const outFile = `${figmaDir}/captures/${job.name}.od-figma.json`;
    writeFileSync(outFile, JSON.stringify(ir));
    // gather fonts
    (ir.fonts || []).forEach(f => { const s = fontsUnion.get(f.family) || new Set(); (f.styles || []).forEach(x => s.add(x)); fontsUnion.set(f.family, s); });
    // add to combined board
    const w = ir.root.width || job.width || 1440;
    const rootCopy = JSON.parse(JSON.stringify(ir.root));
    translateX(rootCopy, cursorX);
    rootCopy.name = `${job.name} — ${ir.source.title || ''}`.trim();
    boardChildren.push(rootCopy);
    cursorX += w + GAP;
    report.push({ name: job.name, file: `captures/${job.name}.od-figma.json`, url: job.url, ...v, fonts: (ir.fonts || []).map(f => f.family), viewport: ir.source.viewport, title: ir.source.title });
    process.stdout.write(`OK   ${job.name.padEnd(28)} nodes=${v.nodeCount} valid=${v.ok}\n`);
  } catch (e) {
    report.push({ name: job.name, file: null, ok: false, errs: [String(e)], nodeCount: 0 });
    process.stdout.write(`FAIL ${job.name}  ${e}\n`);
  }
}

// combined super-root board (genuine — composed of the per-page captures)
const boardHeight = Math.max(1, ...boardChildren.map(c => c.height || 0));
const board = {
  version: 1,
  source: { url: 'thready://all-screens', title: 'Helix Thready — All Screens', capturedAt: Date.now(), viewport: { width: cursorX, height: boardHeight }, dpr: 2 },
  fonts: [...fontsUnion.entries()].map(([family, styles]) => ({ family, styles: [...styles] })),
  root: { type: 'FRAME', name: 'Helix Thready — All Screens', x: 0, y: 0, width: cursorX, height: boardHeight, children: boardChildren },
};
const boardV = validateIR(board);
writeFileSync(`${figmaDir}/thready.od-figma.json`, JSON.stringify(board));

writeFileSync(`${figmaDir}/capture-manifest.json`, JSON.stringify({
  generatedAt: new Date().toISOString(),
  producer: 'open-design/clipper/capture.js → window.__odCapture (injected headless via CDP)',
  irContract: 'figma-plugin/IR.md (version 1)',
  consumer: 'OD Figma Import plugin (figma-plugin/code.js)',
  perPage: report,
  combined: { file: 'thready.od-figma.json', ...boardV, pages: boardChildren.length },
}, null, 2));

const okCount = report.filter(r => r.ok).length;
process.stdout.write(`\ncaptured ${okCount}/${jobs.length} valid; combined board valid=${boardV.ok} nodes=${boardV.nodeCount}\n`);

cli.ws.close(); chrome.kill('SIGTERM'); setTimeout(() => chrome.kill('SIGKILL'), 1500);
process.exit(report.every(r => r.ok) && boardV.ok ? 0 : 1);
