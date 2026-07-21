<!--
  Title           : Helix Thready — Service Discovery & Dynamic Ports
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/architecture/service-discovery.md
  Status          : Draft — v0.1
  Revision        : 1 (2026-07-21)
  Author          : Helix Thready documentation swarm (System Architecture)
  Related         : ./system-overview.md, ./component-catalog.md, ./security-model.md
-->

# Helix Thready — Service Discovery & Dynamic Ports

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-21 | swarm (System Architecture) | Initial draft — discovery, mDNS, port_prefix, health |

## Table of Contents

1. [Requirement](#1-requirement)
2. [Three collaborating submodules](#2-three-collaborating-submodules)
3. [Deterministic dynamic ports (`port_prefix`, verified)](#3-deterministic-dynamic-ports-port_prefix-verified)
4. [Registration & resolution (`discovery` + `mdns`)](#4-registration--resolution-discovery--mdns)
5. [Discovery diagram](#5-discovery-diagram)
6. [Health checks](#6-health-checks)
7. [Subdomain routing across three environments](#7-subdomain-routing-across-three-environments)
8. [Config surface](#8-config-surface)
9. [Gap-register coverage](#9-gap-register-coverage)
10. [TDD reproduce-first skeletons](#10-tdd-reproduce-first-skeletons)
11. [Open items](#11-open-items)

---

## 1. Requirement

The system must support **dynamic port assignment with predefined defaults**, **service
discovery** so every micro-service and infrastructure part finds the others consistently, and
**domain/subdomain binding** so publicly exposed services answer on the right host+port
`[research_request_final §14.3, §8.2]` `[request "Services discovery", "Dynamic ports"]`. All
three environments (dev/sta/prod) run as fully separated container stacks on one Hetzner host
under rootless Podman Compose `[CONSTITUTION §11.4.76]`.

## 2. Three collaborating submodules

| Submodule | Role | Provenance |
|-----------|------|------------|
| `port_prefix` | Deterministic, overflow-safe host-port bands per environment | `[IN-HOUSE: port_prefix]` VERIFIED |
| `digital.vasic.discovery` | Service scan/registry (`pkg/scanner`, `pkg/report`, `pkg/broadcast`, `pkg/resilience`) | `[IN-HOUSE: discovery]` VERIFIED |
| `digital.vasic.mdns` | mDNS advertise/browse for LAN/in-host resolution | `[IN-HOUSE: mdns]` VERIFIED |

Health is provided by `observability/pkg/health`; TLS per subdomain by `lets_encrypt`.

## 3. Deterministic dynamic ports (`port_prefix`, verified)

`port_prefix` maps each service's internal port onto a **numeric band** so ports are dynamic
but *predictable*, never random. Read at source (README, VERIFIED):

```
API: portprefix.Exposed(prefix, internalPort int, taken map[int]bool) (int, error)

For prefix 52, every exposed port is in 52000–52999 (always starts with "52",
always <= 65535); collisions resolved by linear probe within the band.
  443 -> 52443,  80 -> 52080,  8080 -> 52080(taken)->52081,  3000 -> 52000
```

Thready assigns one **prefix band per environment** so the three stacks never collide on the
host:

| Environment | Prefix band | Example: REST(:8443) → host |
|-------------|-------------|------------------------------|
| Development | `52` (52000–52999) | 52443 |
| Staging | `53` (53000–53999) | 53443 |
| Production | `54` (54000–54999) | 54443 |

```go
// Allocate host ports for a service, deterministically, within the env band.
taken := map[int]bool{}
band := 54 // prod
restHost, _ := portprefix.Exposed(band, 8443, taken); taken[restHost] = true
wsHost,  _  := portprefix.Exposed(band, 8080, taken); taken[wsHost]  = true
// restHost=54443, wsHost=54080 — stable across restarts, no clashes with dev/sta bands.
```

Because the mapping is a pure function of `(prefix, internalPort, taken)`, a service's host
port is reproducible from config alone — the deployment scripts and the reverse proxy compute
the same numbers without a lookup, which is what "predefined defaults + dynamic" means here.

## 4. Registration & resolution (`discovery` + `mdns`)

On startup each service **registers** `{name, env, host, port, health_url, tags}` into the
`discovery` registry and **advertises** over mDNS (`_thready._tcp` service type, TXT record
carrying env + role). Peers **resolve** by logical name, never by hard-coded port. The
`discovery` package's scanner/report types are VERIFIED:

```go
// digital.vasic.discovery/pkg/report — VERIFIED at source
type Report struct {
    ScanTime   time.Time
    Duration   time.Duration
    Network    string
    Services   []*scanner.Service
    TotalFound int
}
```

```go
// Thready service self-registration (illustrative composition).
reg := discovery.NewRegistry(discovery.Config{Env: "prod"})
_ = reg.Register(ctx, discovery.Service{
    Name: "semantic-search", Env: "prod",
    Host: hostIP, Port: restHost,               // from port_prefix
    HealthURL: fmt.Sprintf("http://%s:%d/health/ready", hostIP, restHost),
    Tags: []string{"role=search", "grp=processing"},
})
mdns.Advertise(ctx, "_thready._tcp", "semantic-search", restHost,
    map[string]string{"env": "prod", "role": "search"})

// Resolve a dependency by name (Processing → Semantic-search):
svc, err := reg.Resolve(ctx, "semantic-search", "prod")
client := searchclient.New(svc.BaseURL())
```

## 5. Discovery diagram

```mermaid
flowchart TB
  subgraph Host["Hetzner host — rootless Podman Compose (env band)"]
    PP[port_prefix\nExposed(prefix, internalPort, taken)]:::c
    subgraph Envs["env → prefix band"]
      DEV[dev → 52xxx]:::b
      STA[sta → 53xxx]:::b
      PRD[prod → 54xxx]:::b
    end
    REG[(discovery registry\n+ mDNS advertise)]:::db
    HC[observability/pkg/health\n/health/live /health/ready]:::c
  end
  SVC1[Ingestion] --> PP
  SVC2[Processing] --> PP
  SVC3[Asset Service] --> PP
  PP --> DEV & STA & PRD
  SVC1 & SVC2 & SVC3 -->|register name,host,port,health| REG
  SVC1 & SVC2 & SVC3 -->|expose| HC
  REG -->|browse/resolve| SVC2
  RP[Reverse proxy\ndev./sta./thready. subdomains]:::c --> REG
  classDef c fill:#1f6f43,stroke:#0c3b22,color:#eafff0;
  classDef db fill:#124a63,stroke:#062634,color:#e6f6ff;
  classDef b fill:#B6E376,stroke:#2f5d0b,color:#12210a;
```

> Rendered PNG/SVG exported via Docs Chain (§11.4.65). Source: `diagrams/discovery.mmd`.

**Explanation (for readers/models that cannot see the diagram).** Every service (Ingestion,
Processing, Asset Service, …) asks `port_prefix` for its host port; `port_prefix` maps each
internal port into the band for the service's environment — dev into 52xxx, staging into 53xxx,
production into 54xxx — guaranteeing the three stacks never fight over a host port. Each service
then registers `{name, host, port, health}` into the discovery registry and advertises the same
over mDNS, and exposes health endpoints via `observability/pkg/health`. A dependent service
(here Processing) resolves peers by logical name through the registry rather than a hard-coded
address. The reverse proxy at the host edge reads the registry to route the three public
subdomains (`dev.`/`sta.`/`thready.`) to the correct stack. The whole loop means a service's
address is derivable from config, discoverable at runtime, and health-gated before traffic is
sent to it.

## 6. Health checks

Each container exposes liveness and readiness via `observability/pkg/health`
`[research_request_final §22.10]`:

- `GET /health/live` — process is up (cheap; used by Podman restart policy).
- `GET /health/ready` — dependencies reachable (DB, JetStream, storage); gates registry
  advertisement and reverse-proxy routing. A service that fails `ready` is deregistered and
  removed from the proxy pool (failover).

The registry runs periodic health scans (`discovery/pkg/scanner` → `pkg/report`) and marks a
service unreachable after N failed probes, emitting the sticky `channel.health`/service-health
event ([event-model.md](./event-model.md)) so dashboards reflect it in real time.

## 7. Subdomain routing across three environments

One host, one reverse proxy, three isolated container stacks `[research_request_final §21.5]`:

```
dev.thready.hxd3v.com  → dev  stack  (band 52xxx)  — Let's Encrypt cert #1
sta.thready.hxd3v.com  → sta  stack  (band 53xxx)  — Let's Encrypt cert #2
thready.hxd3v.com      → prod stack  (band 54xxx)  — Let's Encrypt cert #3
```

Each subdomain has its own `lets_encrypt` certificate (HTTP-01/DNS-01, auto-renew, atomic
deploy-hook + rollback) `[IN-HOUSE: lets_encrypt]`. The proxy resolves the upstream host:port
from the discovery registry for that env, so a container restart (new nothing — port is stable
by `port_prefix`) or a scale event is transparent.

## 8. Config surface

Discovery/ports are env-driven (documented in the dedicated env-vars doc):

```yaml
# per-environment (illustrative)
THREADY_ENV: prod
THREADY_PORT_PREFIX: 54          # port_prefix band
THREADY_DISCOVERY_ADDR: 127.0.0.1:5400
THREADY_MDNS_SERVICE: _thready._tcp
THREADY_HEALTH_PATH: /health/ready
THREADY_PUBLIC_DOMAIN: thready.hxd3v.com
```

## 9. Gap-register coverage

No `[GAP: …]` items are owned by discovery/ports directly (the register flags no discovery
gaps). The relevant cross-cutting item is `[GAP: 3.2]` (database partitioning at scale), which
is a data-plane concern handled in [data-flow.md](./data-flow.md), not discovery. The
constitution's decoupling-audit `[CONSTITUTION §11.4.28]` applies: `discovery`, `mdns`,
`port_prefix` are consumed config-injected, never forked.

## 10. TDD reproduce-first skeletons

```go
// RED: port_prefix must be deterministic and collision-free within a band.
func TestPortPrefix_Deterministic(t *testing.T) {
    taken := map[int]bool{}
    a, _ := portprefix.Exposed(54, 8443, taken); taken[a] = true
    b, _ := portprefix.Exposed(54, 8443, map[int]bool{a: true})
    require.Equal(t, 54443, a)
    require.NotEqual(t, a, b) // second caller probes to next free
}

// RED: a service failing readiness is removed from the registry.
func TestRegistry_DeregisterOnUnhealthy(t *testing.T) {
    reg := newRegistry(t); reg.Register(ctx, unhealthySvc)
    reg.RunHealthScan(ctx)
    _, err := reg.Resolve(ctx, unhealthySvc.Name, "prod")
    require.ErrorIs(t, err, discovery.ErrNoHealthyInstance)
}

// RED: two envs must not collide on a host port.
func TestBands_NoCrossEnvCollision(t *testing.T) {
    dev, _ := portprefix.Exposed(52, 8443, map[int]bool{})
    prod, _ := portprefix.Exposed(54, 8443, map[int]bool{})
    require.NotEqual(t, dev, prod) // 52443 vs 54443
}
```

## 11. Open items

- `[OPEN: DISC-1]` Exact `discovery.Registry` constructor/method names (`NewRegistry`,
  `Register`, `Resolve`) are illustrative — the VERIFIED surface read this pass was
  `pkg/report`/`pkg/scanner`; the registration API (`pkg/broadcast`?) must be source-confirmed
  before wiring. Tracked in the re-verification backlog.
- `[OPEN: DISC-2]` Whether the reverse proxy is Caddy/nginx/Traefik or a `vasic-digital`
  component is a deployment-pack decision; the discovery contract above is proxy-agnostic.

---

*Made with love ♥ by Helix Development.*
