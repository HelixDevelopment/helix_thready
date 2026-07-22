#!/usr/bin/env node
// -----------------------------------------------------------------------------
// Helix Thready — headless render driver (PNG screenshots + genuine OD Figma IR)
// Location: docs/public/research/mvp/design/scripts/render-cdp.mjs
//
// Drives /usr/bin/chromium (headless) over the Chrome DevTools Protocol using
// node's built-in global WebSocket/fetch (node 24). No puppeteer/playwright.
//
//   * PNG  : screenshots every screen artifact in BOTH light + dark themes at
//            deviceScaleFactor=2 (web 1440 full-page, mobile clipped to the
//            .device 390px frame, desktop/tui natural full-page).
//   * FIGMA: injects OpenDesign's OWN clipper/capture.js and serialises
//            window.__odCapture().figmaIr → <area>-<screen>.od-figma.json.
//            This is byte-identical to the OD Clipper "Download Figma (.json)"
//            producer (clipper/background.js downloadFigma). A genuine capture,
//            not hand-authored plugin output.
//
// Theme is forced with document.documentElement.setAttribute('data-theme', …)
// before capture (the artifacts' CSS keys dark off :root[data-theme="dark"]).
// -----------------------------------------------------------------------------
import { spawn } from 'node:child_process';
import { mkdtempSync, existsSync, readFileSync, writeFileSync, mkdirSync, readdirSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, dirname } from 'node:path';
import { pathToFileURL } from 'node:url';

const CHROMIUM = '/usr/bin/chromium';
const DES = '/home/milos/Factory/projects/tools_and_research/helix_thready/docs/public/research/mvp/design';
const CAPTURE_JS = readFileSync(
  '/home/milos/Factory/projects/tools_and_research/.opendesign-src/open-design/clipper/capture.js',
  'utf8',
);
const OUT = join(DES, 'exports');
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// ---- Job list ---------------------------------------------------------------
const WEB = ['accounts-admin','assets-browser','billing','channels','dashboard','events-monitor',
  'login','post-detail','research-viewer','search','settings','skills-manager','thread-explorer'];
const MOBILE = ['account','channel-threads','home-feed','notifications','post-detail','search','settings'];

const screens = [];
for (const s of WEB)    screens.push({ area:'web',    name:s, file:join(DES,'screens/web',s+'.html'),    width:1440, mobile:false, clip:null });
for (const s of MOBILE) screens.push({ area:'mobile', name:s, file:join(DES,'screens/mobile',s+'.html'), width:1040, mobile:true,  clip:'.device' });
screens.push({ area:'desktop', name:'desktop-shell', file:join(DES,'screens/desktop/desktop-shell.html'), width:1440, mobile:false, clip:null });
screens.push({ area:'tui',     name:'tui-screens',   file:join(DES,'screens/tui/tui-screens.html'),       width:1040, mobile:false, clip:null });

// PNG-only screen artifacts (rendered light + dark, but NOT sent through the OD
// Figma IR capture — index.html is a live-preview gallery and components.html is
// the component library sheet; both are mandated as screenshots, not frames).
const pngOnly = [
  { area:'web',     name:'index',      file:join(DES,'screens/web/index.html'),  width:1440, mobile:false, clip:null },
  { area:'library', name:'components', file:join(DES,'library/components.html'),  width:1280, mobile:false, clip:null },
];

// Bonus renders for the PDF book (NOT part of the mandatory screen-PNG count).
const extras = [
  { area:'motion',  name:'preview',        file:join(DES,'motion/preview.html'),     width:1280, mobile:false, clip:null, themes:['light'] },
];

// ---- Minimal CDP client -----------------------------------------------------
class CDP {
  constructor(wsUrl){ this.ws=new WebSocket(wsUrl); this.id=0; this.pending=new Map(); this.handlers=new Map(); }
  connect(){ return new Promise((res,rej)=>{
    this.ws.addEventListener('open',()=>res());
    this.ws.addEventListener('error',(e)=>rej(new Error('ws error '+(e && e.message||''))));
    this.ws.addEventListener('message',(ev)=>{
      const m=JSON.parse(ev.data);
      if(m.id && this.pending.has(m.id)){ const {resolve,reject}=this.pending.get(m.id); this.pending.delete(m.id);
        if(m.error) reject(new Error(JSON.stringify(m.error))); else resolve(m.result); }
      else if(m.method){ (this.handlers.get(m.method)||[]).forEach((h)=>h(m.params)); }
    });
  }); }
  send(method,params={}){ const id=++this.id; return new Promise((resolve,reject)=>{
    this.pending.set(id,{resolve,reject}); this.ws.send(JSON.stringify({id,method,params})); }); }
  on(method,cb){ if(!this.handlers.has(method)) this.handlers.set(method,[]); this.handlers.get(method).push(cb); }
  once(method,timeoutMs){ return new Promise((res)=>{ let done=false;
    const cb=(p)=>{ if(done)return; done=true; const arr=this.handlers.get(method); const i=arr.indexOf(cb); if(i>=0)arr.splice(i,1); res(p); };
    this.on(method,cb); if(timeoutMs) setTimeout(cb,timeoutMs); }); }
}

async function launchChromium(){
  const udd = mkdtempSync(join(tmpdir(),'od-cdp-'));
  const args = ['--headless=new','--disable-gpu','--no-sandbox','--no-first-run','--no-default-browser-check',
    '--hide-scrollbars','--force-color-profile=srgb','--disable-lcd-text',
    '--remote-debugging-port=0',`--user-data-dir=${udd}`,'about:blank'];
  const proc = spawn(CHROMIUM,args,{stdio:['ignore','ignore','pipe']});
  proc.stderr.on('data',()=>{}); // silence
  // Wait for DevToolsActivePort file
  const portFile = join(udd,'DevToolsActivePort');
  let port=null;
  for(let i=0;i<100;i++){ if(existsSync(portFile)){ const c=readFileSync(portFile,'utf8').split('\n'); if(c[0]){ port=c[0].trim(); break; } } await sleep(100); }
  if(!port) throw new Error('chromium did not expose a debugging port');
  // Find a page target
  let wsUrl=null;
  for(let i=0;i<50;i++){
    const list = await (await fetch(`http://127.0.0.1:${port}/json/list`)).json();
    const page = list.find((t)=>t.type==='page');
    if(page && page.webSocketDebuggerUrl){ wsUrl=page.webSocketDebuggerUrl; break; }
    await sleep(100);
  }
  if(!wsUrl){ // create one
    await fetch(`http://127.0.0.1:${port}/json/new?about:blank`,{method:'PUT'}).catch(()=>{});
    const list = await (await fetch(`http://127.0.0.1:${port}/json/list`)).json();
    const page = list.find((t)=>t.type==='page'); wsUrl = page && page.webSocketDebuggerUrl;
  }
  if(!wsUrl) throw new Error('no page target');
  const cdp = new CDP(wsUrl); await cdp.connect();
  await cdp.send('Page.enable'); await cdp.send('Runtime.enable');
  return { proc, cdp, port };
}

async function loadPage(cdp,file,width,mobile,theme){
  await cdp.send('Emulation.setDeviceMetricsOverride',{
    width, height:900, deviceScaleFactor:2, mobile:!!mobile,
    screenWidth:width, screenHeight:900, dontSetVisibleSize:false,
  });
  const url = pathToFileURL(file).href;
  const loaded = cdp.once('Page.loadEventFired',15000);
  await cdp.send('Page.navigate',{url});
  await loaded;
  // Force theme deterministically (overrides the media-query default). Also stamp
  // any same-origin iframes (index.html embeds a live-preview iframe of a screen).
  await cdp.send('Runtime.evaluate',{expression:
    `(function(){try{localStorage.setItem('thready-theme','${theme}');}catch(e){}`+
    `document.documentElement.setAttribute('data-theme','${theme}');`+
    `try{document.querySelectorAll('iframe').forEach(function(f){try{`+
    `var d=f.contentDocument;if(d&&d.documentElement){d.documentElement.setAttribute('data-theme','${theme}');}`+
    `}catch(e){}});}catch(e){}`+
    `return document.documentElement.getAttribute('data-theme');})()`,
    returnByValue:true});
  // Fonts ready + settle for any entry animation.
  await cdp.send('Runtime.evaluate',{expression:
    `(document.fonts&&document.fonts.ready)?document.fonts.ready.then(()=>1):Promise.resolve(1)`,
    awaitPromise:true}).catch(()=>{});
  await sleep(650);
}

async function measureClip(cdp,clipSel){
  if(clipSel){
    const r=await cdp.send('Runtime.evaluate',{returnByValue:true,expression:
      `(function(){var el=document.querySelector(${JSON.stringify(clipSel)});if(!el)return null;`+
      `var b=el.getBoundingClientRect();return{x:Math.max(0,b.left+window.scrollX-4),y:Math.max(0,b.top+window.scrollY-4),`+
      `width:Math.ceil(b.width)+8,height:Math.ceil(b.height)+8};})()`});
    if(r.result && r.result.value) return r.result.value;
  }
  const m=await cdp.send('Page.getLayoutMetrics');
  const cs=m.cssContentSize||m.contentSize;
  return {x:0,y:0,width:Math.ceil(cs.width),height:Math.ceil(cs.height)};
}

async function screenshot(cdp,file,width,mobile,theme,clipSel,out){
  await loadPage(cdp,file,width,mobile,theme);
  const clip=await measureClip(cdp,clipSel);
  const shot=await cdp.send('Page.captureScreenshot',{format:'png',captureBeyondViewport:true,
    clip:{x:clip.x,y:clip.y,width:clip.width,height:clip.height,scale:1}});
  mkdirSync(dirname(out),{recursive:true});
  writeFileSync(out,Buffer.from(shot.data,'base64'));
  return clip;
}

async function captureFigma(cdp,file,width,mobile,out){
  await loadPage(cdp,file,width,mobile,'light');
  // Inject OpenDesign's own capture.js (defines window.__odCapture), then run it.
  await cdp.send('Runtime.evaluate',{expression:CAPTURE_JS});
  const r=await cdp.send('Runtime.evaluate',{returnByValue:true,awaitPromise:true,expression:
    `(function(){var c=window.__odCapture({includeImages:false});`+
    `return JSON.stringify({ir:c.figmaIr,nodes:c.figmaNodeCount,truncated:c.figmaTruncated});})()`});
  const parsed=JSON.parse(r.result.value);
  mkdirSync(dirname(out),{recursive:true});
  writeFileSync(out,JSON.stringify(parsed.ir,null,2));
  return {nodes:parsed.nodes,truncated:parsed.truncated,version:parsed.ir&&parsed.ir.version};
}

// ---- Main -------------------------------------------------------------------
(async()=>{
  const { proc, cdp } = await launchChromium();
  const report={png:[],figma:[],errors:[]};
  try{
    // 1) PNG screenshots — light + dark (mandatory screens + index + components)
    for(const s of [...screens, ...pngOnly]){
      for(const theme of ['light','dark']){
        const suffix = theme==='dark' ? '-dark' : '';
        const out=join(OUT,'png',s.area,`${s.name}${suffix}.png`);
        try{
          const clip=await screenshot(cdp,s.file,s.width,s.mobile,theme,s.clip,out);
          report.png.push({area:s.area,name:s.name,theme,out,clip});
          process.stderr.write(`PNG  ok  ${s.area}/${s.name}${suffix}  ${clip.width}x${clip.height}css\n`);
        }catch(e){ report.errors.push({kind:'png',area:s.area,name:s.name,theme,error:String(e.message||e)});
          process.stderr.write(`PNG  ERR ${s.area}/${s.name}${suffix}: ${e.message||e}\n`); }
      }
    }
    // 1b) Bonus renders for the book
    for(const s of extras){
      for(const theme of (s.themes||['light'])){
        const suffix = theme==='dark' ? '-dark' : '';
        const out=join(OUT,'png',s.area,`${s.name}${suffix}.png`);
        try{ const clip=await screenshot(cdp,s.file,s.width,s.mobile,theme,s.clip,out);
          report.png.push({area:s.area,name:s.name,theme,out,clip,extra:true});
          process.stderr.write(`PNG  ok  ${s.area}/${s.name}${suffix} (extra)  ${clip.width}x${clip.height}css\n`);
        }catch(e){ report.errors.push({kind:'png-extra',area:s.area,name:s.name,theme,error:String(e.message||e)}); }
      }
    }
    // 2) Genuine OD Figma IR captures
    for(const s of screens){
      const out=join(OUT,'figma','captures',`${s.area}-${s.name}.od-figma.json`);
      try{
        const meta=await captureFigma(cdp,s.file,s.width,s.mobile,out);
        report.figma.push({area:s.area,name:s.name,out,...meta});
        process.stderr.write(`FIG  ok  ${s.area}-${s.name}  nodes=${meta.nodes} v${meta.version} trunc=${meta.truncated}\n`);
      }catch(e){ report.errors.push({kind:'figma',area:s.area,name:s.name,error:String(e.message||e)});
        process.stderr.write(`FIG  ERR ${s.area}-${s.name}: ${e.message||e}\n`); }
    }
  } finally {
    writeFileSync(join(OUT,'pdf-build','render-report.json'),JSON.stringify(report,null,2));
    try{ await cdp.send('Browser.close'); }catch(e){}
    proc.kill('SIGTERM');
  }
  process.stdout.write(JSON.stringify({pngCount:report.png.length,figmaCount:report.figma.length,errors:report.errors.length})+'\n');
})().catch((e)=>{ console.error('FATAL',e); process.exit(1); });
