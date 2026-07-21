<!--
  Title           : Helix Thready — Hetzner Host Provisioning
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/deployment/hetzner-provisioning.md
  Status          : Review — v0.2
  Revision        : 2 (2026-07-21)
  Author          : Helix Thready documentation swarm (deployment)
  Related         : ./index.md, ./podman-compose.md, ./secrets-and-config.md,
                    ./tls-lets-encrypt.md, ./environments.md
-->

# Helix Thready — Hetzner Host Provisioning

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-21 | swarm (deployment) | Initial root→thready provisioning, rootless Podman, firewall, linger, sysctl, first deploy |
| 2 | 2026-07-21 | swarm (deployment review) | Split the provisioning-phases prose into multiple paragraphs |

This document is the step-by-step provisioning of the single Hetzner dedicated host: from **root**
bootstrap to the unprivileged **`thready`** user under which all three environments run rootless
(`§14.2`, Q8). It ends at a first health-gated deploy.

> Diagram source: sibling under [`diagrams/`](./diagrams/). Rendered PNG/SVG exported via Docs Chain (§11.4.65).

## Table of Contents

1. [Host baseline (sizing)](#1-host-baseline-sizing)
2. [Provisioning phases diagram](#2-provisioning-phases-diagram)
3. [Phase A — root bootstrap (one time)](#3-phase-a--root-bootstrap-one-time)
4. [The privileged-port sysctl (why root is needed once)](#4-the-privileged-port-sysctl-why-root-is-needed-once)
5. [Firewall](#5-firewall)
6. [Phase B — the thready user (rootless)](#6-phase-b--the-thready-user-rootless)
7. [Phase C — first deploy](#7-phase-c--first-deploy)
8. [Host toolchain (incl. docs_chain deps)](#8-host-toolchain-incl-docs_chain-deps)
9. [Verified vs assumed](#9-verified-vs-assumed)
10. [Open items](#10-open-items)

---

## 1. Host baseline (sizing)

`[DEFAULT — adjustable]` baseline for the Large scale (Q5) — three co-resident stacks + object tier:

| Resource | Baseline | Rationale |
|----------|----------|-----------|
| vCPU | ≥ 16 | prod + sta + dev app/data planes co-reside |
| RAM | ≥ 64 GB | Postgres + pgvector working set; JetStream; three stacks |
| Disk | NVMe for Postgres/pgvector; large HDD/NVMe pool for MinIO (50 TB+) | Q5, asset tier |
| Network | 1 Gbit+; static IPv4 + IPv6 | edge on one IP; HTTP/3 over UDP |
| GPU | **not on this host** — HelixLLM runs on the workstation/GPU node | Q5; reached as external endpoint |

`[OPEN: host-sizing]` — exact Hetzner SKU (e.g. a dedicated **AX**/**EX** line box) and whether MinIO
uses local disks or a Hetzner volume/Storage Box for the object tier is finalized after load tests.
HelixLLM's GPU node topology is separate.

## 2. Provisioning phases diagram

```mermaid
flowchart TD
  subgraph ROOT["Phase A — as root (one time)"]
    A1[Update OS + install podman, podman-compose, rclone, openssl, curl, pandoc, weasyprint]
    A2[Create user 'thready' home /home/thready]
    A3[Set /etc/subuid /etc/subgid for thready]
    A4[sysctl net.ipv4.ip_unprivileged_port_start=80]
    A5[Harden SSH: key-only, no root login]
    A6[Firewall nftables: allow 22,80,443; deny rest]
    A7[loginctl enable-linger thready]
    A1-->A2-->A3-->A4-->A5-->A6-->A7
  end
  subgraph USER["Phase B — as thready (rootless)"]
    B1[SSH key-in as thready]
    B2[Clone helix_thready + submodules containers, lets_encrypt]
    B3[Place .env + api_keys.sh chmod 600 from private repo]
    B4[podman login registries if needed]
    B5[lets_encrypt setup.sh --install acme.sh rootless]
    B6[Install systemd --user timers: le-renew-{dev,sta,prod}]
    B1-->B2-->B3-->B4-->B5-->B6
  end
  subgraph FIRST["Phase C — first deploy"]
    C1[Issue certs STAGING then PROD per subdomain]
    C2[deploy-stack / thready-deploy.sh per env]
    C3[Health gate + edge probe]
    C4[Enable stack systemd --user units]
    C1-->C2-->C3-->C4
  end
  A7-->B1
  B6-->C1
```

**Explanation (for readers/models that cannot see the diagram).** Provisioning has three phases.
**Phase A (root, one time)** does everything that genuinely needs privilege and nothing more: it
updates the OS and installs the container + backup + cert + docs toolchain; creates the unprivileged
`thready` user with home `/home/thready`; assigns the subordinate UID/GID ranges rootless Podman
needs (`/etc/subuid`, `/etc/subgid`); lowers the `net.ipv4.ip_unprivileged_port_start` sysctl to 80
so a rootless container can bind 80/443; hardens SSH to key-only with no root login; installs an
`nftables` firewall that allows only 22/80/443; and enables **lingering** for `thready` so the user's
systemd (stacks + cert timers) runs even when nobody is logged in. After Phase A, root is **no longer
used** in the runtime path.

**Phase B (as `thready`, rootless)** signs in with an SSH key, clones the `helix_thready` repo plus
the `containers` and `lets_encrypt` submodules, drops the runtime secrets (`.env`, `api_keys.sh`)
with `chmod 600` from the private repo, logs into any container registries, installs `acme.sh`
rootless via the `lets_encrypt` setup script, and installs the per-environment renewal timers. Every
command in this phase runs as the unprivileged user — there is no `sudo` anywhere in it.

**Phase C (first deploy)** issues certificates against the staging CA then production per subdomain,
deploys each environment through the health-gated deploy, and enables the stacks' `systemd --user`
units so they survive reboot. The arrows enforce order: linger before the user phase, timers before
certs, certs before the app deploy — so nothing is attempted before its prerequisite exists.

## 3. Phase A — root bootstrap (one time)

Run **once** as root on a fresh Hetzner host. Idempotent where possible.

```bash
#!/usr/bin/env bash
# provision-root.sh  (run as root, ONE TIME)
set -euo pipefail

# 1. OS + toolchain (Debian/Ubuntu shown; adapt for the chosen distro)
apt-get update && apt-get -y upgrade
apt-get -y install podman podman-compose slirp4netns fuse-overlayfs \
                   rclone openssl curl git nftables \
                   pandoc weasyprint            # docs_chain deps — GAP #19

# 2. Create the unprivileged runtime user
useradd -m -s /bin/bash -d /home/thready thready

# 3. Rootless Podman subordinate ID ranges (65536 each is the usual default)
usermod --add-subuids 100000-165535 --add-subgids 100000-165535 thready

# 4. Allow rootless bind of 80/443 (see §4 for the why)
echo 'net.ipv4.ip_unprivileged_port_start=80' > /etc/sysctl.d/99-thready-ports.conf
sysctl --system

# 5. Harden SSH (key-only, no root login)
install -m 700 -o thready -g thready -d /home/thready/.ssh
# (append the operator's public key to /home/thready/.ssh/authorized_keys, chmod 600)
sed -i 's/^#\?PermitRootLogin.*/PermitRootLogin no/' /etc/ssh/sshd_config
sed -i 's/^#\?PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config
systemctl reload ssh

# 6. Firewall (see §5)
install -m 600 /dev/stdin /etc/nftables.conf <<'NFT'
# … see §5 …
NFT
systemctl enable --now nftables

# 7. Let the thready user's systemd run without an active login (stacks + cert timers)
loginctl enable-linger thready
```

> The SSH credentials for the `thready` account are provided at deployment time and kept **only in
> the private submodule** (`§14.2`); they never enter a public repo or a log.

## 4. The privileged-port sysctl (why root is needed once)

Rootless containers cannot bind ports < 1024 by default. Thready needs the edge on 80 and 443. Two
options; Thready picks the first as its `[DEFAULT — adjustable]`:

1. **Lower `net.ipv4.ip_unprivileged_port_start` to 80** (chosen) — a single, auditable sysctl set
   once by root in Phase A. Thereafter the rootless `thready` edge binds 80/443 with **no** runtime
   privilege. Clean and simple.
2. **Port-forward helper / socket activation** — keep the sysctl at 1024 and forward 80/443 via a
   privileged helper or `systemd` socket. More moving parts; avoided for MVP.

Only the edge needs this; every other service binds a high loopback port in its
[band](./service-discovery-ports.md). This keeps the privileged-port surface to exactly two ports on
one container.

## 5. Firewall

Minimal ingress — the entire public surface is SSH + the edge:

```nft
# /etc/nftables.conf
table inet filter {
  chain input {
    type filter hook input priority 0; policy drop;
    ct state established,related accept
    iif "lo" accept
    ip protocol icmp accept
    ip6 nexthdr icmpv6 accept
    tcp dport 22 accept          # SSH (key-only)
    tcp dport { 80, 443 } accept # edge: HTTP-01 + HTTPS
    udp dport 443 accept         # HTTP/3 (QUIC) — edge
  }
  chain forward { type filter hook forward priority 0; policy drop; }
  chain output  { type filter hook output priority 0; policy accept; }
}
```

- **No** database/MinIO/NATS port is ever open to the internet — they bind `127.0.0.1` in their
  bands and are reached only via the compose network or an operator SSH tunnel.
- `udp/443` is required for HTTP/3 (QUIC), which the edge and `vasic-digital/http3` use.
- Optional: `fail2ban` on SSH; egress stays open for ACME, image pulls and messenger APIs.

## 6. Phase B — the thready user (rootless)

```bash
#!/usr/bin/env bash
# provision-thready.sh  (run as the thready user, rootless)
set -euo pipefail
cd /home/thready

# 1. Clone the project + the two deployment submodules
git clone git@github.com:HelixDevelopment/helix_thready.git repo
git -C repo submodule update --init submodules/containers submodules/lets_encrypt || true
ln -sfn repo/submodules/containers   /home/thready/submodules/containers
ln -sfn repo/submodules/lets_encrypt /home/thready/submodules/lets_encrypt

# 2. Layout (see podman-compose.md §3)
mkdir -p edge/certs/{dev,sta,prod} dev/config sta/config prod/config secrets releases

# 3. Runtime secrets from the PRIVATE repo — chmod 600, never committed here
#    (see secrets-and-config.md). Placeholder: operator copies them in securely.
for e in dev sta prod; do install -m 600 /dev/null "$e/.env"; done
install -m 600 /dev/null secrets/api_keys.sh

# 4. Rootless acme.sh install (per env config)
for e in prod sta dev; do
  submodules/lets_encrypt/scripts/setup.sh --config "$e/config/lets_encrypt.conf" --install
done

# 5. Install the rootless renewal timers (see tls-lets-encrypt.md §10)
#    le-renew-{dev,sta,prod}.timer under ~/.config/systemd/user/
systemctl --user daemon-reload
```

- Everything runs as `thready`; **no `sudo`** appears anywhere in this phase.
- `podman login` (if pulling from a private registry) stores creds under the user's runtime dir.

## 7. Phase C — first deploy

```bash
# As thready. Staging certs first (rehearse), then production.
for e in prod sta dev; do
  # a. issue certs (LE_STAGING=1 → 0 after rehearsal), per subdomain
  submodules/lets_encrypt/scripts/issue.sh --config "$e/config/lets_encrypt.conf"
done

# b. bring up the edge, then each env via the health-gated deploy
cd edge && podman-compose -p thready-edge up -d && cd ..
for e in dev sta prod; do
  submodules/containers/scripts/thready-deploy.sh "$e" "THREADY-$(cat repo/VERSION)"
done

# c. enable stack + edge units so they survive reboot (linger already on)
systemctl --user enable --now thready-edge.service \
  thready-dev.service thready-sta.service thready-prod.service
```

The deploy in step (b) is the full [health-gated pipeline with rollback](./deploy-and-rollback.md);
a failed env restores itself and does not block the others.

## 8. Host toolchain (incl. docs_chain deps)

`[GAP: #19 docs_chain]` — the `docs_chain` engine honestly **SKIPs** md→HTML/PDF sibling generation
when `pandoc`/`weasyprint` are absent (the gap register notes the current host lacks them). Phase A
therefore **installs `pandoc` and `weasyprint`** so every `.md` in this pack gets its HTML/PDF
siblings per `[CONSTITUTION §11.4.65]`. The full host toolchain:

| Tool | Purpose |
|------|---------|
| `podman`, `podman-compose`, `slirp4netns`/`pasta`, `fuse-overlayfs` | rootless container runtime |
| `rclone` | ship backups to the secondary store |
| `acme.sh` (via `lets_encrypt setup.sh`) | ACME client (installed rootless in Phase B) |
| `openssl`, `curl` | cert validation + reachability probes (lets_encrypt deps) |
| `pandoc`, `weasyprint` | **docs_chain** md→HTML/PDF (`[GAP: #19]`) |
| `git` | repo + submodules + all-upstreams push (`§2.1`) |
| `nftables` | firewall |

## 9. Verified vs assumed

- **VERIFIED:** the root→`thready` user model with home `/home/thready` (`§14.2`, Q8); rootless
  mandate (`§11.4.76/161`); `lets_encrypt setup.sh --install` is rootless (module README); the
  docs_chain pandoc/weasyprint SKIP behaviour (gap register #19); private-repo-only SSH creds
  (`§14.2`).
- **ASSUMED / `[DEFAULT — adjustable]`:** the sysctl-lowering approach for privileged ports; the
  Debian/Ubuntu package names; the `nftables` ruleset; the subuid/subgid ranges; the ≥16 vCPU/≥64 GB
  sizing baseline.

## 10. Open items

- `[OPEN: host-sizing]` — exact Hetzner SKU + MinIO object-tier backing (local disks vs volume vs
  Storage Box) + GPU-node topology, pending load tests.
- `[OPEN: distro]` — the host distro (Debian/Ubuntu/AlmaLinux) is a `[DEFAULT — adjustable]` choice;
  package names in Phase A adapt accordingly.
- `[OPEN: golden-image]` — a saved post-Phase-A image would shorten the [DR runbook](./backup-dr.md)
  step 1; nice-to-have, not MVP-blocking.

---

*Made with love ♥ by Helix Development.*
