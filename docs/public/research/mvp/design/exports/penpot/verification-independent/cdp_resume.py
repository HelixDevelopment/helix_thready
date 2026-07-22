#!/usr/bin/env python3
"""Resume screenshot capture from a given file slug, fresh tab per file, retries."""
import json, time, sys, os
import cdp_shots as m

HERE = os.path.dirname(os.path.abspath(__file__))
start_from = sys.argv[1] if len(sys.argv) > 1 else "03-screens-web"

files = json.load(open(os.path.join(HERE, "files.json")))
team, proj = files["team"], files["project"]
slugs = list(files["files"].keys())
todo = slugs[slugs.index(start_from):]

for slug in todo:
    fid = files["files"][slug]
    for attempt in (1, 2, 3):
        try:
            c = m.CDP()          # fresh ws + fresh tab (cookies persist in browser profile)
            c.open_page()
            c.goto(f"{m.BASE}/#/workspace?team-id={team}&file-id={fid}", settle=6)
            c.wait_js("!!document.querySelector('.viewport, [class*=viewport], canvas')", 60)
            time.sleep(15)
            c.shot(f"file-{slug}.png")
            print(f"  {slug} attempt {attempt}: OK | url:", c.js("location.href"))
            try:
                c.cmd("Target.closeTarget",
                      {"targetId": c.cmd("Target.getTargetInfo")["targetInfo"]["targetId"]})
            except Exception:
                pass
            break
        except Exception as e:
            print(f"  {slug} attempt {attempt}: FAIL {e}")
            time.sleep(5)
print("resume done")
