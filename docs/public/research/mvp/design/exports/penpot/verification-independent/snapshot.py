#!/usr/bin/env python3
"""Independent verifier snapshot: polls PenPot RPC and records per-file counts.
Appends one JSON line per snapshot to snapshots.jsonl in the same directory.
"""
import json, sys, urllib.request, datetime, os

BASE = "http://localhost:9001/api/rpc/command"
HERE = os.path.dirname(os.path.abspath(__file__))
CRED = "/home/milos/Factory/projects/tools_and_research/helix_thready/.penpot-credentials"

token = None
with open(CRED) as f:
    for line in f:
        if line.startswith("AccessToken"):
            token = line.split(": ", 1)[1].strip()
assert token, "token not found"

def rpc(cmd, **params):
    qs = "&".join(f"{k}={v}" for k, v in params.items())
    url = f"{BASE}/{cmd}" + (f"?{qs}" if qs else "")
    req = urllib.request.Request(url, headers={
        "Authorization": f"Token {token}", "Accept": "application/json"})
    with urllib.request.urlopen(req, timeout=120) as r:
        return json.load(r)

def summarize_file(fid):
    try:
        f = rpc("get-file", id=fid)
    except Exception as e:
        return {"error": str(e)}
    data = f.get("data") or {}
    pages = data.get("pagesIndex") or {}
    page_names = []
    total_objects = 0
    frames = 0
    for pid, page in pages.items():
        page_names.append(page.get("name", "?"))
        objs = page.get("objects") or {}
        total_objects += len(objs)
        frames += sum(1 for o in objs.values()
                      if o.get("type") == "frame" and o.get("id") != "00000000-0000-0000-0000-000000000000")
    comps = data.get("components") or {}
    colors = data.get("colors") or {}
    typos = data.get("typographies") or {}
    tlib = data.get("tokensLib") or {}
    # DTCG-style: top-level set names (non-$ keys), $themes list, $metadata
    tsets = {k: v for k, v in tlib.items() if not k.startswith("$")}
    def _count(o):
        if isinstance(o, dict):
            if "$value" in o or "value" in o:
                return 1
            return sum(_count(v) for v in o.values())
        return 0
    token_count = sum(_count(v) for v in tsets.values())
    set_names = sorted(tsets.keys())
    theme_names = [f"{t.get('group','')}/{t.get('name','')}"
                   for t in (tlib.get("$themes") or [])]
    media = f.get("mediaObjects") or []
    return {
        "name": f.get("name"), "revn": f.get("revn"),
        "modifiedAt": f.get("modifiedAt"),
        "pages": len(pages), "pageNames": page_names,
        "objects": total_objects, "frames": frames,
        "components": len(comps),
        "colors": len(colors), "typographies": len(typos),
        "tokenSets": len(tsets), "tokenSetNames": set_names,
        "tokenThemes": len(theme_names), "tokenThemeNames": theme_names,
        "tokens": token_count,
        "mediaObjects": len(media) if isinstance(media, list) else media,
    }

def main():
    ts = datetime.datetime.now(datetime.timezone.utc).isoformat()
    team = rpc("get-teams")[0]["id"]
    projects = rpc("get-projects", **{"team-id": team})
    snap = {"ts": ts, "projects": []}
    for p in projects:
        pe = {"id": p["id"], "name": p["name"], "fileCount": p.get("count")}
        if p["name"] == "Helix Thready":
            files = rpc("get-project-files", **{"project-id": p["id"]})
            pe["files"] = {}
            for f in sorted(files, key=lambda x: x["name"]):
                pe["files"][f["name"]] = summarize_file(f["id"])
                pe["files"][f["name"]]["id"] = f["id"]
        snap["projects"].append(pe)
    with open(os.path.join(HERE, "snapshots.jsonl"), "a") as out:
        out.write(json.dumps(snap) + "\n")
    # compact stdout summary
    for p in snap["projects"]:
        if "files" not in p:
            continue
        print(f"[{ts}] project={p['name']} files={p['fileCount']}")
        for name, s in p["files"].items():
            if "error" in s:
                print(f"  {name}: ERROR {s['error']}")
            else:
                print(f"  {name}: revn={s['revn']} pages={s['pages']} objects={s['objects']} "
                      f"frames={s['frames']} components={s['components']} colors={s['colors']} "
                      f"typos={s['typographies']} tokenSets={s['tokenSets']} tokens={s['tokens']} "
                      f"themes={s['tokenThemes']}")

if __name__ == "__main__":
    main()
