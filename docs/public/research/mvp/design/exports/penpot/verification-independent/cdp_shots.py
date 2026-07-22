#!/usr/bin/env python3
"""Independent verifier: drive headless-shell via raw CDP.
Login to PenPot UI, screenshot dashboard + each file workspace."""
import json, time, base64, sys, os, urllib.request
import websocket

CDP_HTTP = "http://127.0.0.1:9333"
BASE = "http://localhost:9001"
HERE = os.path.dirname(os.path.abspath(__file__))
CRED = "/home/milos/Factory/projects/tools_and_research/helix_thready/.penpot-credentials"

email = password = None
with open(CRED) as f:
    for line in f:
        if line.startswith("Email"):
            email = line.split(":", 1)[1].strip()
        elif line.startswith("Password"):
            password = line.split(":", 1)[1].strip()

class CDP:
    def __init__(self):
        v = json.load(urllib.request.urlopen(f"{CDP_HTTP}/json/version"))
        self.ws = websocket.create_connection(v["webSocketDebuggerUrl"], timeout=60,
                                              suppress_origin=True)
        self.mid = 0
        self.session = None

    def cmd(self, method, params=None, session=None):
        self.mid += 1
        m = {"id": self.mid, "method": method, "params": params or {}}
        sid = session or self.session
        if sid:
            m["sessionId"] = sid
        self.ws.send(json.dumps(m))
        deadline = time.time() + 90
        while time.time() < deadline:
            r = json.loads(self.ws.recv())
            if r.get("id") == self.mid:
                if "error" in r:
                    raise RuntimeError(f"{method}: {r['error']}")
                return r.get("result", {})
        raise TimeoutError(method)

    def open_page(self):
        t = self.cmd("Target.createTarget", {"url": "about:blank"})
        a = self.cmd("Target.attachToTarget", {"targetId": t["targetId"], "flatten": True},
                     session="")
        self.session = a["sessionId"]
        self.cmd("Page.enable")
        self.cmd("Runtime.enable")
        self.cmd("Emulation.setDeviceMetricsOverride",
                 {"width": 1440, "height": 900, "deviceScaleFactor": 1, "mobile": False})

    def js(self, expr, await_promise=False):
        r = self.cmd("Runtime.evaluate", {"expression": expr, "returnByValue": True,
                                          "awaitPromise": await_promise})
        return r.get("result", {}).get("value")

    def goto(self, url, settle=6):
        self.cmd("Page.navigate", {"url": url})
        time.sleep(settle)

    def wait_js(self, expr, timeout=60, poll=1.5):
        deadline = time.time() + timeout
        while time.time() < deadline:
            if self.js(expr):
                return True
            time.sleep(poll)
        return False

    def shot(self, name):
        r = self.cmd("Page.captureScreenshot", {"format": "png"})
        p = os.path.join(HERE, name)
        with open(p, "wb") as f:
            f.write(base64.b64decode(r["data"]))
        print(f"saved {name} ({os.path.getsize(p)} bytes)")

def main():
    c = CDP()
    c.open_page()

    # --- login via UI ---
    c.goto(f"{BASE}/#/auth/login", settle=8)
    ok = c.wait_js("!!document.querySelector('input[name=email], input[type=email]')", 40)
    print("login form present:", ok)
    fill = """
    (function(){
      var e = document.querySelector('input[name=email], input[type=email]');
      var p = document.querySelector('input[name=password], input[type=password]');
      if(!e||!p) return 'no-inputs';
      var set = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype,'value').set;
      set.call(e, %s); e.dispatchEvent(new Event('input',{bubbles:true}));
      set.call(p, %s); p.dispatchEvent(new Event('input',{bubbles:true}));
      return 'filled';
    })()
    """ % (json.dumps(email), json.dumps(password))
    print("fill:", c.js(fill))
    time.sleep(1)
    print("submit:", c.js("""
    (function(){
      var b = document.querySelector('button[type=submit], input[type=submit]');
      if(!b) return 'no-button';
      b.click(); return 'clicked';
    })()"""))
    ok = c.wait_js("location.hash.includes('dashboard')", 45)
    print("dashboard reached:", ok, "| hash:", c.js("location.hash"))
    time.sleep(6)  # let project cards + thumbnails render
    c.shot("dashboard.png")

    # --- project page (Helix Thready) ---
    files = json.load(open(os.path.join(HERE, "files.json")))
    team = files["team"]
    proj = files["project"]
    c.goto(f"{BASE}/#/dashboard/files/{team}/{proj}", settle=10)
    c.shot("dashboard-project-helix-thready.png")

    # --- each file workspace ---
    for slug, fid in files["files"].items():
        url = f"{BASE}/#/workspace?team-id={team}&file-id={fid}"
        c.goto(url, settle=5)
        # wait for workspace viewport svg/canvas to exist, then settle for render
        c.wait_js("!!document.querySelector('.viewport, [class*=viewport], canvas')", 60)
        time.sleep(12)
        c.shot(f"file-{slug}.png")
        print("  url now:", c.js("location.href"))

    c.cmd("Target.closeTarget", {"targetId": c.cmd("Target.getTargets", session='')["targetInfos"][0]["targetId"]}, session="") if False else None
    print("done")

if __name__ == "__main__":
    main()
