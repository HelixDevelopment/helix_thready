#!/usr/bin/env node
// cdp-render.mjs — headless-Chromium screenshot renderer over the DevTools
// Protocol using Node's built-in WebSocket (Node >= 22). No npm deps.
// Forces theme via Emulation.setEmulatedMedia + a pre-load localStorage seed;
// deviceScaleFactor=2; captures full page height.  Usage: node cdp-render.mjs <manifest.json>

import { spawn } from 'node:child_process';
import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname } from 'node:path';
import { setTimeout as sleep } from 'node:timers/promises';

const manifest = JSON.parse(await (await import('node:fs/promises')).readFile(process.argv[2], 'utf8'));
const CHROMIUM = manifest.chromium || 'chromium';
const PORT = manifest.port || 9333;
const SETTLE = manifest.settleMs ?? 700;

const userDataDir = `/tmp/cdp-render-${process.pid}`;
const chrome = spawn(CHROMIUM, [
  '--headless=new', `--remote-debugging-port=${PORT}`, `--user-data-dir=${userDataDir}`,
  '--no-sandbox', '--disable-gpu', '--hide-scrollbars', '--no-first-run',
  '--disable-extensions', '--force-color-profile=srgb', '--allow-file-access-from-files',
  '--disable-lcd-text', 'about:blank',
], { stdio: ['ignore', 'ignore', 'pipe'] });
let stderr = ''; chrome.stderr.on('data', d => { stderr += d.toString(); });

async function getWsUrl() {
  for (let i = 0; i < 100; i++) {
    try { const r = await fetch(`http://127.0.0.1:${PORT}/json/version`); if (r.ok) return (await r.json()).webSocketDebuggerUrl; } catch {}
    await sleep(100);
  }
  throw new Error('chromium devtools endpoint never came up\n' + stderr);
}
function connect(url) {
  const ws = new WebSocket(url); const pending = new Map(); const listeners = []; let id = 0;
  const ready = new Promise((res, rej) => { ws.onopen = () => res(); ws.onerror = e => rej(e); });
  ws.onmessage = ev => { const msg = JSON.parse(ev.data);
    if (msg.id && pending.has(msg.id)) { const { resolve, reject } = pending.get(msg.id); pending.delete(msg.id); msg.error ? reject(new Error(JSON.stringify(msg.error))) : resolve(msg.result); }
    else { for (const l of listeners) l(msg); } };
  const send = (method, params = {}, sessionId) => new Promise((resolve, reject) => { const mid = ++id; pending.set(mid, { resolve, reject }); ws.send(JSON.stringify({ id: mid, method, params, ...(sessionId ? { sessionId } : {}) })); });
  return { ws, ready, send, onEvent: fn => listeners.push(fn) };
}
const cli = connect(await getWsUrl());
await cli.ready;

async function renderJob(job) {
  const { targetId } = await cli.send('Target.createTarget', { url: 'about:blank' });
  const { sessionId } = await cli.send('Target.attachToTarget', { targetId, flatten: true });
  const S = (m, p) => cli.send(m, p, sessionId);
  let loadFired = false;
  cli.onEvent(msg => { if (msg.sessionId === sessionId && msg.method === 'Page.loadEventFired') loadFired = true; });
  await S('Page.enable'); await S('Runtime.enable');
  await S('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-color-scheme', value: job.theme === 'dark' ? 'dark' : 'light' }] });
  const seed = job.theme === 'dark' ? 'dark' : 'light';
  await S('Page.addScriptToEvaluateOnNewDocument', { source: `(function(){try{localStorage.setItem('thready-theme','${seed}');}catch(e){}try{document.documentElement.setAttribute('data-theme','${seed}');}catch(e){}})();` });
  const width = job.width || 1440;
  await S('Emulation.setDeviceMetricsOverride', { width, height: 900, deviceScaleFactor: 2, mobile: !!job.mobile, screenWidth: width, screenHeight: 900 });
  loadFired = false;
  await S('Page.navigate', { url: job.url });
  for (let i = 0; i < 150 && !loadFired; i++) await sleep(50);
  await sleep(SETTLE);
  await S('Runtime.evaluate', { expression: `try{document.documentElement.setAttribute('data-theme','${seed}');}catch(e){}` });
  const h = await S('Runtime.evaluate', { expression: 'Math.max(document.body?document.body.scrollHeight:0,document.documentElement.scrollHeight,document.documentElement.offsetHeight)', returnByValue: true });
  let fullHeight = Math.ceil(h.result.value || 900); if (fullHeight < 200) fullHeight = 900;
  const capHeight = Math.min(fullHeight, 20000);
  await S('Emulation.setDeviceMetricsOverride', { width, height: capHeight, deviceScaleFactor: 2, mobile: !!job.mobile, screenWidth: width, screenHeight: capHeight });
  await sleep(120);
  // deviceScaleFactor=2 already yields the 2x density; clip.scale MUST be 1 here,
  // otherwise the two multiply and the capture comes out at 4x (2*2).
  const shot = await S('Page.captureScreenshot', { format: 'png', captureBeyondViewport: true, clip: { x: 0, y: 0, width, height: capHeight, scale: 1 } });
  mkdirSync(dirname(job.out), { recursive: true });
  writeFileSync(job.out, Buffer.from(shot.data, 'base64'));
  const probe = await S('Runtime.evaluate', { expression: `(function(){var b=getComputedStyle(document.body||document.documentElement);return JSON.stringify({dt:document.documentElement.getAttribute('data-theme'),bg:b.backgroundColor});})()`, returnByValue: true });
  await cli.send('Target.closeTarget', { targetId });
  return { out: job.out, height: capHeight, width: width * 2, ...JSON.parse(probe.result.value) };
}

const results = [];
for (const job of manifest.jobs) {
  try { const r = await renderJob(job); results.push({ ok: true, ...r }); process.stdout.write(`OK   ${job.theme.padEnd(5)} ${job.out}  ${r.width}x${r.height*2}px  bg=${r.bg}\n`); }
  catch (e) { results.push({ ok: false, out: job.out, error: String(e) }); process.stdout.write(`FAIL ${job.out}  ${e}\n`); }
}
writeFileSync(manifest.reportOut || '/tmp/cdp-render-report.json', JSON.stringify(results, null, 2));
cli.ws.close(); chrome.kill('SIGTERM'); setTimeout(() => chrome.kill('SIGKILL'), 1500);
process.exit(results.every(r => r.ok) ? 0 : 1);
