#!/usr/bin/env node
/*
 * Helix Thready — token-bridge codegen (THREADY-DES-LIB-04 / -KMP-01 / -FLUT-01 / -TUI-01)
 *
 * Parses ../opendesign/tokens.css — THE canonical token source — into an
 * internal model and deterministically emits every per-platform binding
 * promised by design-system.md §7. No npm dependencies (Node >= 18).
 *
 * The Lipgloss (Go) target additionally reads the normative mapping tables of
 * ../screens/tui/lipgloss-theme.md §2 (truecolor → ANSI-256 → ANSI-16) and
 * FAILS if that doc's truecolor column drifts from tokens.css.
 *
 * The PenPot target (generated/penpot/tokens.penpot-import.json) is the
 * PenPot-2.17-shaped projection of the same DTCG model: token sets + $themes,
 * with NO root-level $description — a top-level $description flips PenPot
 * 2.17's tokens importer into single-set mode (one set "tokens", colors only,
 * no themes) [VERIFIED 2026-07-22 — real import]. See README §9.
 *
 * Usage:
 *   node generate.mjs           (re)generate ./generated/**
 *   node generate.mjs --check   regenerate into a temp dir, byte-diff against
 *                               the committed outputs, and run the cross-check
 *                               suite (hex round-trip, structural self-checks,
 *                               JSON parse, doc-vs-css consistency). Exit != 0
 *                               on any drift or failure — CI-able.
 *
 * Determinism: output order follows tokens.css declaration order; the only
 * variable content is the sha256 of tokens.css itself (embedded in every
 * header so a reader can tell which source revision produced the file).
 */

import { readFileSync, writeFileSync, mkdirSync, mkdtempSync, existsSync, renameSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { dirname, join, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';
import { tmpdir } from 'node:os';

const HERE = dirname(fileURLToPath(import.meta.url));
const TOKENS_CSS_PATH = resolve(HERE, '..', 'opendesign', 'tokens.css');
const LIPGLOSS_DOC_PATH = resolve(HERE, '..', 'screens', 'tui', 'lipgloss-theme.md');
// Operator-derived file that ACTUALLY imported into PenPot 2.17 on 2026-07-22.
// While it exists, --check structurally diffs the native penpot target against it.
const PENPOT_REFERENCE_PATH = resolve(HERE, '..', 'exports', 'penpot', 'tokens.penpot-import.json');
const OUT_ROOT = join(HERE, 'generated');

/* ────────────────────────────────────────────────────────────────────────────
 * 1. Parse tokens.css into the model
 * ──────────────────────────────────────────────────────────────────────────── */

function stripComments(css) {
  return css.replace(/\/\*[\s\S]*?\*\//g, '');
}

function parseDecls(body) {
  const out = [];
  for (const part of body.split(';')) {
    const t = part.trim();
    if (!t) continue;
    const idx = t.indexOf(':');
    if (idx < 0) continue;
    const name = t.slice(0, idx).trim();
    const value = t.slice(idx + 1).trim();
    if (name.startsWith('--')) out.push([name.slice(2), value]);
  }
  return out;
}

function extractBlock(css, headRe, label) {
  const m = css.match(headRe);
  if (!m) throw new Error(`tokens.css: cannot locate ${label} block`);
  return m[1];
}

const HEX_RE = /^#([0-9a-fA-F]{6})$/;
const ALIAS_RE = /^var\(--([a-z0-9-]+)\)$/;

function buildModel() {
  const raw = readFileSync(TOKENS_CSS_PATH);
  const hash = createHash('sha256').update(raw).digest('hex');
  const css = stripComments(raw.toString('utf8'));

  const lightDecls = parseDecls(extractBlock(css, /(?<!\S):root\s*\{([^}]*)\}/, ':root (light)'));
  const darkMedia = parseDecls(extractBlock(
    css,
    /@media\s*\(prefers-color-scheme:\s*dark\)\s*\{\s*:root:not\(\[data-theme="light"\]\)\s*\{([^}]*)\}/,
    '@media dark',
  ));
  const darkExplicit = parseDecls(extractBlock(
    css,
    /:root\[data-theme="dark"\]\s*,\s*\.dark\s*\{([^}]*)\}/,
    '[data-theme="dark"], .dark',
  ));

  // Consistency gate: the two dark blocks must bind identical name→value sets.
  const asKeyed = (decls) => JSON.stringify(Object.fromEntries(decls.map(([k, v]) => [k, v.toLowerCase()])));
  if (asKeyed(darkMedia) !== asKeyed(darkExplicit)) {
    throw new Error('tokens.css drift: the @media dark block and the [data-theme="dark"]/.dark block disagree');
  }
  const dark = new Map(darkMedia.map(([k, v]) => [k, v]));

  const model = {
    hash,
    colors: [],      // { name, light, dark }             physical hex, both modes
    aliases: [],     // { name, target }                  var(--x) B-slot/C-ext aliases
    dims: [],        // { name, px }                      px dimensions
    numbers: [],     // { name, value }                   unitless (line-heights)
    tracking: [],    // { name, em }                      em letter-spacing
    durations: [],   // { name, ms }
    easings: [],     // { name, points }                  cubic-bezier
    fonts: [],       // { name, families }
    themeId: null,
    skipped: [],     // CSS-only formula/shadow tokens (color-mix / var-composites)
  };

  for (const [name, value] of lightDecls) {
    let m;
    if (name === 'theme-id') {
      model.themeId = value.replace(/"/g, '');
    } else if ((m = value.match(HEX_RE))) {
      const dv = dark.get(name);
      if (dv === undefined) throw new Error(`tokens.css: color --${name} has no dark re-bind`);
      const dm = dv.match(HEX_RE);
      if (!dm) throw new Error(`tokens.css: dark value of --${name} is not a plain hex: ${dv}`);
      model.colors.push({ name, light: m[1].toLowerCase(), dark: dm[1].toLowerCase() });
    } else if ((m = value.match(ALIAS_RE))) {
      model.aliases.push({ name, target: m[1] });
    } else if ((m = value.match(/^(-?\d+(?:\.\d+)?)px$/))) {
      model.dims.push({ name, px: Number(m[1]) });
    } else if ((m = value.match(/^(-?\d+(?:\.\d+)?)ms$/))) {
      model.durations.push({ name, ms: Number(m[1]) });
    } else if ((m = value.match(/^(-?\d+(?:\.\d+)?)em$/))) {
      model.tracking.push({ name, em: Number(m[1]) });
    } else if ((m = value.match(/^(-?\d+(?:\.\d+)?)$/))) {
      model.numbers.push({ name, value: Number(m[1]) });
    } else if ((m = value.match(/^cubic-bezier\(([^)]*)\)$/))) {
      model.easings.push({ name, points: m[1].split(',').map((s) => Number(s.trim())) });
    } else if (value.includes(',') && /^"/.test(value)) {
      model.fonts.push({ name, families: value.split(',').map((s) => s.trim().replace(/^"|"$/g, '')) });
    } else {
      // color-mix formulas, shadow composites, `none` — CSS-runtime only.
      model.skipped.push(name);
    }
  }

  // Every alias must resolve to a physical color token.
  for (const a of model.aliases) {
    if (!model.colors.some((c) => c.name === a.target)) {
      throw new Error(`tokens.css: alias --${a.name} targets unknown token --${a.target}`);
    }
  }
  // Every dark binding must correspond to a light declaration.
  for (const k of dark.keys()) {
    if (!model.colors.some((c) => c.name === k) && !model.aliases.some((a) => a.name === k)) {
      throw new Error(`tokens.css: dark block binds --${k} which the light block does not declare`);
    }
  }
  return model;
}

/* ────────────────────────────────────────────────────────────────────────────
 * 2. Parse the normative TUI mapping (lipgloss-theme.md §2)
 * ──────────────────────────────────────────────────────────────────────────── */

function parseLipglossDoc(model) {
  const text = readFileSync(LIPGLOSS_DOC_PATH, 'utf8');
  const rows = [];
  for (const line of text.split('\n')) {
    if (!line.startsWith('|')) continue;
    const cells = line.split('|').map((s) => s.trim());
    // | Role | CSS token | Truecolor (dark) | provenance | ANSI-256 | ANSI-16 | Mapping provenance |
    if (cells.length < 9) continue;
    const token = (cells[2].match(/`--([a-z0-9-]+)`/) || [])[1];
    const hex = (cells[3].match(/`#([0-9A-Fa-f]{6})`/) || [])[1];
    const a256 = (cells[5].match(/`(\d+)`/) || [])[1];
    const a16m = cells[6].match(/`(\d+)`\s*(.*)$/);
    if (!token || !hex || !a256 || !a16m) continue;
    rows.push({
      role: cells[1],
      token,
      hex: hex.toLowerCase(),
      ansi256: a256,
      ansi16: a16m[1],
      ansi16Name: a16m[2].trim(),
      mapping: cells[7],
    });
  }
  if (rows.length === 0) throw new Error('lipgloss-theme.md: §2 mapping table not found');
  // Normative consistency gate: doc truecolor column must equal tokens.css dark values.
  for (const r of rows) {
    const c = model.colors.find((c) => c.name === r.token);
    if (!c) throw new Error(`lipgloss-theme.md maps unknown token --${r.token}`);
    if (c.dark !== r.hex) {
      throw new Error(
        `DRIFT: lipgloss-theme.md says --${r.token} = #${r.hex} but tokens.css (dark) = #${c.dark}`,
      );
    }
  }
  return rows;
}

/* ────────────────────────────────────────────────────────────────────────────
 * 3. Naming + formatting helpers
 * ──────────────────────────────────────────────────────────────────────────── */

const segs = (kebab) => kebab.split('-');
const cap = (s) => (s ? s[0].toUpperCase() + s.slice(1) : s);
const pascal = (kebab) => segs(kebab).map(cap).join('');
const camel = (kebab) => {
  const p = pascal(kebab);
  return p[0].toLowerCase() + p.slice(1);
};
const up = (hex) => hex.toUpperCase();
const spaceShort = (name) => (name.startsWith('space-') ? 's' + name.slice(6) : camel(name));
const radiusShort = (name) => name.replace(/^radius-/, '');
const f6 = (n) => n.toFixed(6);
const chan = (hex, i) => parseInt(hex.slice(i * 2, i * 2 + 2), 16);
const fmtNum = (n) => String(n);
// ASCII-escaped JSON (\uXXXX for every non-ASCII char, 2-space indent) — the
// exact serialization of the file that proved the PenPot 2.17 import.
const asciiJson = (v) =>
  JSON.stringify(v, null, 2).replace(/[\u0080-\uffff]/g, (ch) => '\\u' + ch.charCodeAt(0).toString(16).padStart(4, '0'));

function header(model, comment, extra = []) {
  const lines = [
    'GENERATED — DO NOT EDIT.',
    'Generated by tokens-bridge/generate.mjs from ../opendesign/tokens.css',
    `sha256(tokens.css) = ${model.hash}`,
    'Single source of truth: opendesign/tokens.css (design-system.md §3/§7).',
    `Not exported (CSS-runtime color-mix()/var() formulas): ${model.skipped.map((n) => '--' + n).join(', ')}.`,
    ...extra,
  ];
  return lines.map((l) => `${comment} ${l}`.trimEnd()).join('\n');
}

/* ────────────────────────────────────────────────────────────────────────────
 * 4. Emitters — each returns { relPath, content, colors:Set, symbolRe, symbols }
 * ──────────────────────────────────────────────────────────────────────────── */

function modelColorSet(model) {
  const s = new Set();
  for (const c of model.colors) {
    s.add(up(c.light));
    s.add(up(c.dark));
  }
  return s;
}

/* 4.1 W3C DTCG design tokens (canonical) + PenPot 2.17 import projection.
 *
 * PenPot 2.17 quirks [VERIFIED 2026-07-22 — real import, README §9]:
 *   (1) a top-level `$description` member flips the tokens importer into
 *       single-set mode (one set named "tokens", 38 colors, no themes). The
 *       penpot target therefore carries NO root-level doc/metadata member —
 *       which is also why it is the one generated file WITHOUT a sha256
 *       header: any root `$`-doc member is exactly the quirk-triggering shape,
 *       and `$metadata` extensions are unproven in 2.17.
 *   (2) `duration` (×2) and `cubicBezier` (×1) tokens are silently dropped on
 *       import — 71/74 land. They are INCLUDED regardless (mirroring the
 *       operator-derived file that proved the import; lossless DTCG-side) and
 *       reported via the "penpot-unsupported" note at run time.
 */

// Theme ids pinned to the proven 2026-07-22 PenPot 2.17 import — determinism:
// regeneration must never mint new UUIDs.
const PENPOT_THEMES = [
  {
    id: '0072a4d2-7f66-4fb0-a4df-1246beaed082',
    name: 'Light',
    group: 'Thready',
    selectedTokenSets: { 'thready-light': 'enabled', 'thready-structure': 'enabled' },
  },
  {
    id: '5d457c10-223e-4b72-ad9b-988eedb95af2',
    name: 'Dark',
    group: 'Thready',
    selectedTokenSets: { 'thready-dark': 'enabled', 'thready-structure': 'enabled' },
  },
];

function penpotUnsupported(model) {
  return [
    ...model.durations.map((d) => `--${d.name} (duration)`),
    ...model.easings.map((e) => `--${e.name} (cubicBezier)`),
  ];
}

// The three token sets (thready-light / thready-dark / thready-structure),
// shared verbatim by the web (DTCG) and penpot targets.
function buildDtcgSets(model) {
  const sets = {};
  let symbols = 0;
  for (const mode of ['light', 'dark']) {
    const group = { color: {} };
    for (const c of model.colors) {
      group.color[c.name] = {
        $type: 'color',
        $value: `#${c[mode]}`,
        $description: `--${c.name} (${mode}) [VERIFIED — tokens.css]`,
      };
      symbols += 1;
    }
    for (const a of model.aliases) {
      group.color[a.name] = {
        $type: 'color',
        $value: `{thready-${mode}.color.${a.target}}`,
        $description: `--${a.name} → var(--${a.target}) (alias) [VERIFIED — tokens.css]`,
      };
      symbols += 1;
    }
    sets[`thready-${mode}`] = group;
  }
  const structure = { dimension: {}, number: {}, duration: {}, cubicBezier: {}, fontFamily: {} };
  for (const d of model.dims) {
    structure.dimension[d.name] = { $type: 'dimension', $value: `${fmtNum(d.px)}px` };
    symbols += 1;
  }
  for (const n of model.numbers) {
    structure.number[n.name] = { $type: 'number', $value: n.value };
    symbols += 1;
  }
  for (const t of model.tracking) {
    structure.number[t.name] = {
      $type: 'number',
      $value: t.em,
      $description: `em letter-spacing (CSS: ${fmtNum(t.em)}em) — DTCG dimension allows only px/rem, so carried as a number of em`,
    };
    symbols += 1;
  }
  for (const d of model.durations) {
    structure.duration[d.name] = { $type: 'duration', $value: `${fmtNum(d.ms)}ms` };
    symbols += 1;
  }
  for (const e of model.easings) {
    structure.cubicBezier[e.name] = { $type: 'cubicBezier', $value: e.points };
    symbols += 1;
  }
  for (const f of model.fonts) {
    structure.fontFamily[f.name] = { $type: 'fontFamily', $value: f.families };
    symbols += 1;
  }
  sets['thready-structure'] = structure;
  return { sets, symbols };
}

const dtcgExtractColors = (s) =>
  new Set([...s.matchAll(/"\$value": "#([0-9a-fA-F]{6})"/g)].map((m) => up(m[1])));

function emitDtcg(model) {
  const { sets, symbols } = buildDtcgSets(model);
  const doc = {
    $description:
      'GENERATED — DO NOT EDIT. Helix Thready design tokens in W3C DTCG format — ' +
      'the canonical interchange artifact. NOT the PenPot import file: PenPot 2.17 ' +
      'treats a top-level $description as single-set mode [VERIFIED 2026-07-22] — ' +
      'import generated/penpot/tokens.penpot-import.json instead. ' +
      'Generated by tokens-bridge/generate.mjs ' +
      `from ../opendesign/tokens.css, sha256(tokens.css) = ${model.hash}. ` +
      'Color modes are the token groups thready-light / thready-dark. ' +
      'Not exported (CSS-runtime color-mix()/var() formulas): ' +
      model.skipped.map((n) => '--' + n).join(', ') +
      '. --theme-id carried as group description: theme-id = ' + model.themeId + '.',
    ...sets,
  };
  return {
    relPath: 'web/tokens.json',
    content: JSON.stringify(doc, null, 2) + '\n',
    colors: modelColorSet(model),
    extractColors: dtcgExtractColors,
    symbolRe: /"\$type":/g,
    symbols,
  };
}

/* 4.1b PenPot 2.17 design-tokens import file — multi-set + $themes shape.
 * Serialized ASCII-escaped, 2-space indent, no trailing newline: byte-for-byte
 * the serialization of the operator-derived file that proved the import. */
function emitPenpot(model) {
  const { sets, symbols } = buildDtcgSets(model);
  const doc = {
    ...sets,
    $themes: PENPOT_THEMES,
    $metadata: { tokenSetOrder: Object.keys(sets) },
  };
  return {
    relPath: 'penpot/tokens.penpot-import.json',
    content: asciiJson(doc),
    colors: modelColorSet(model),
    extractColors: dtcgExtractColors,
    symbolRe: /"\$type":/g,
    symbols,
  };
}

/* 4.2 Compose / KMP (contract for UI-Components-KMP — GAP: 8.4) */
function emitCompose(model) {
  let symbols = 0;
  const val = () => (symbols += 1, 'val');
  const colorObj = (mode) => {
    const lines = [];
    for (const c of model.colors) {
      lines.push(`        ${val()} ${pascal(c.name)} = Color(0xFF${up(c[mode])}) // --${c.name} (${mode})`);
    }
    for (const a of model.aliases) {
      lines.push(`        ${val()} ${pascal(a.name)} = ${pascal(a.target)} // --${a.name} → var(--${a.target})`);
    }
    return lines.join('\n');
  };
  const spacing = model.dims.filter((d) => d.name.startsWith('space-') || d.name.startsWith('section-') || d.name.startsWith('container-'));
  const radius = model.dims.filter((d) => d.name.startsWith('radius-'));
  const type = model.dims.filter((d) => d.name.startsWith('text-'));
  const content = `${header(model, '//', [
    'Contract for UI-Components-KMP [GAP: 8.4] — replaces the hand-kept foreign palette.',
    'Package: design-system.md §7 sample declares no package — digital.vasic.thready.design [DEFAULT — adjustable].',
  ])}

package digital.vasic.thready.design

import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.em
import androidx.compose.ui.unit.sp

const ${(symbols += 1, 'val')} THREADY_THEME_ID = "${model.themeId}" // --theme-id

object ThreadyColors {
    object LightColors {
${colorObj('light')}
    }

    object DarkColors {
${colorObj('dark')}
    }
}

object ThreadySpacing {
${spacing.map((d) => `    ${val()} ${spaceShort(d.name)} = ${fmtNum(d.px)}.dp // --${d.name}`).join('\n')}
}

object ThreadyRadius {
${radius.map((d) => `    ${val()} ${radiusShort(d.name)} = ${fmtNum(d.px)}.dp // --${d.name}`).join('\n')}
}

object ThreadyTypeScale {
${type.map((d) => `    ${val()} ${camel(d.name)} = ${fmtNum(d.px)}.sp // --${d.name}`).join('\n')}
${model.numbers.map((n) => `    ${val()} ${camel(n.name)} = ${n.value}f // --${n.name} (line-height multiplier)`).join('\n')}
${model.tracking.map((t) => `    ${val()} ${camel(t.name)} = (${fmtNum(t.em)}).em // --${t.name}`).join('\n')}
}

object ThreadyMotion {
${model.durations.map((d) => `    const ${val()} ${camel(d.name).replace(/^motion/, '').replace(/^./, (ch) => ch.toUpperCase()).toUpperCase()}_MILLIS = ${fmtNum(d.ms)} // --${d.name}`).join('\n')}
}
`;
  return {
    relPath: 'compose/ThreadyColors.kt',
    content,
    colors: modelColorSet(model),
    extractColors: (s) => new Set([...s.matchAll(/Color\(0xFF([0-9A-F]{6})\)/g)].map((m) => up(m[1]))),
    symbolRe: /\bval /g,
    symbols,
  };
}

/* 4.3 SwiftUI (no in-house SwiftUI package exists — OPEN: THREADY-DES-LIB-02) */
function emitSwift(model) {
  let symbols = 0;
  const slet = () => (symbols += 1, 'public static let');
  const swiftColor = (hex) =>
    `Color(red: ${f6(chan(hex, 0) / 255)}, green: ${f6(chan(hex, 1) / 255)}, blue: ${f6(chan(hex, 2) / 255)})`;
  const colorEnum = (mode) => {
    const lines = [];
    for (const c of model.colors) {
      lines.push(`        ${slet()} ${camel(c.name)} = ${swiftColor(c[mode])} // --${c.name} (${mode}) #${up(c[mode])}`);
    }
    for (const a of model.aliases) {
      lines.push(`        ${slet()} ${camel(a.name)} = ${camel(a.target)} // --${a.name} → var(--${a.target})`);
    }
    return lines.join('\n');
  };
  const spacing = model.dims.filter((d) => d.name.startsWith('space-') || d.name.startsWith('section-') || d.name.startsWith('container-'));
  const radius = model.dims.filter((d) => d.name.startsWith('radius-'));
  const type = model.dims.filter((d) => d.name.startsWith('text-'));
  const content = `${header(model, '//', [
    'Delivery-ready contract only: no in-house SwiftUI package exists [OPEN: THREADY-DES-LIB-02];',
    'the sanctioned iOS path is KMP/Compose (library/platform-map.md §2).',
  ])}

import SwiftUI

public enum ThreadyTokens {
    ${slet()} themeId = "${model.themeId}" // --theme-id

    public enum LightColors {
${colorEnum('light')}
    }

    public enum DarkColors {
${colorEnum('dark')}
    }

    public enum Spacing {
${spacing.map((d) => `        ${slet()} ${spaceShort(d.name)}: CGFloat = ${fmtNum(d.px)} // --${d.name}`).join('\n')}
    }

    public enum Radius {
${radius.map((d) => `        ${slet()} ${radiusShort(d.name)}: CGFloat = ${fmtNum(d.px)} // --${d.name}`).join('\n')}
    }

    public enum TypeScale {
${type.map((d) => `        ${slet()} ${camel(d.name)}: CGFloat = ${fmtNum(d.px)} // --${d.name} (pt)`).join('\n')}
${model.numbers.map((n) => `        ${slet()} ${camel(n.name)}: CGFloat = ${n.value} // --${n.name} (line-height multiplier)`).join('\n')}
${model.tracking.map((t) => `        ${slet()} ${camel(t.name)}: CGFloat = ${fmtNum(t.em)} // --${t.name} (em)`).join('\n')}
    }

    public enum Motion {
${model.durations.map((d) => `        ${slet()} ${camel(d.name)}Ms = ${fmtNum(d.ms)} // --${d.name}`).join('\n')}
    }
}
`;
  return {
    relPath: 'swiftui/ThreadyTokens.swift',
    content,
    colors: modelColorSet(model),
    extractColors: (s) =>
      new Set(
        [...s.matchAll(/Color\(red: ([0-9.]+), green: ([0-9.]+), blue: ([0-9.]+)\)/g)].map((m) =>
          [m[1], m[2], m[3]]
            .map((f) => Math.round(Number(f) * 255).toString(16).padStart(2, '0'))
            .join('')
            .toUpperCase(),
        ),
      ),
    symbolRe: /public static let /g,
    symbols,
  };
}

/* 4.4 ArkTS / HarmonyOS (native path via helix_shims — GAP: 8.5) */
function emitArkts(model) {
  let symbols = 0;
  const sr = () => (symbols += 1, 'static readonly');
  const colorClass = (cls, mode) => {
    const lines = [`export class ${cls} {`];
    for (const c of model.colors) {
      lines.push(`  ${sr()} ${camel(c.name)}: string = '#${up(c[mode])}' // --${c.name} (${mode}), ResourceColor-compatible`);
    }
    for (const a of model.aliases) {
      lines.push(`  ${sr()} ${camel(a.name)}: string = ${cls}.${camel(a.target)} // --${a.name} → var(--${a.target})`);
    }
    lines.push('}');
    return lines.join('\n');
  };
  const spacing = model.dims.filter((d) => d.name.startsWith('space-') || d.name.startsWith('section-') || d.name.startsWith('container-'));
  const radius = model.dims.filter((d) => d.name.startsWith('radius-'));
  const type = model.dims.filter((d) => d.name.startsWith('text-'));
  const content = `${header(model, '//', [
    'Delivery-ready contract only: the ArkTS/HarmonyOS client path is native via helix_shims,',
    'uninspected [GAP: 8.5] [OPEN: THREADY-DES-LIB-03]. Hex strings are ResourceColor-compatible;',
    'spacing/radius numbers are vp, type-scale numbers are fp.',
  ])}

export const THREADY_THEME_ID: string = '${model.themeId}' // --theme-id

${colorClass('ThreadyColorLight', 'light')}

${colorClass('ThreadyColorDark', 'dark')}

export class ThreadySpacing { // vp
${spacing.map((d) => `  ${sr()} ${spaceShort(d.name)}: number = ${fmtNum(d.px)} // --${d.name}`).join('\n')}
}

export class ThreadyRadius { // vp
${radius.map((d) => `  ${sr()} ${radiusShort(d.name)}: number = ${fmtNum(d.px)} // --${d.name}`).join('\n')}
}

export class ThreadyTypeScale { // fp
${type.map((d) => `  ${sr()} ${camel(d.name)}: number = ${fmtNum(d.px)} // --${d.name}`).join('\n')}
${model.numbers.map((n) => `  ${sr()} ${camel(n.name)}: number = ${n.value} // --${n.name} (line-height multiplier, unitless)`).join('\n')}
${model.tracking.map((t) => `  ${sr()} ${camel(t.name)}: number = ${fmtNum(t.em)} // --${t.name} (em, unitless)`).join('\n')}
}

export class ThreadyMotion { // ms
${model.durations.map((d) => `  ${sr()} ${camel(d.name)}Ms: number = ${fmtNum(d.ms)} // --${d.name}`).join('\n')}
}
`;
  return {
    relPath: 'arkts/thready_tokens.ets',
    content,
    colors: modelColorSet(model),
    extractColors: (s) => new Set([...s.matchAll(/'#([0-9A-F]{6})'/g)].map((m) => up(m[1]))),
    symbolRe: /static readonly /g,
    symbols,
  };
}

/* 4.5 QML singleton (Qt / Aurora arm of helix_design — empty scaffold, GAP: 8.2/8.3) */
function emitQml(model) {
  let symbols = 0;
  const rp = () => (symbols += 1, 'readonly property');
  const palette = (mode) => {
    const lines = [];
    for (const c of model.colors) {
      lines.push(`        ${rp()} color ${camel(c.name)}: "#${c[mode]}" // --${c.name} (${mode})`);
    }
    for (const a of model.aliases) {
      lines.push(`        ${rp()} color ${camel(a.name)}: ${camel(a.target)} // --${a.name} → var(--${a.target})`);
    }
    return lines.join('\n');
  };
  const content = `${header(model, '//', [
    'Delivery-ready contract only: the Qt/Aurora arm of helix_design is a verified',
    'empty scaffold [GAP: 8.2/8.3]; Aurora path is native via helix_shims [GAP: 8.5].',
    'Register as a QML singleton (qmldir: singleton ThreadyTokens 1.0 ThreadyTokens.qml).',
  ])}
pragma Singleton
import QtQuick 2.15

QtObject {
    ${rp()} string themeId: "${model.themeId}" // --theme-id

    ${rp()} QtObject light: QtObject {
${palette('light')}
    }

    ${rp()} QtObject dark: QtObject {
${palette('dark')}
    }

    // Structure (px)
${model.dims.map((d) => `    ${rp()} int ${camel(d.name)}: ${fmtNum(d.px)} // --${d.name}`).join('\n')}

    // Line heights (multipliers) + tracking (em)
${model.numbers.map((n) => `    ${rp()} real ${camel(n.name)}: ${n.value} // --${n.name}`).join('\n')}
${model.tracking.map((t) => `    ${rp()} real ${camel(t.name)}: ${fmtNum(t.em)} // --${t.name} (em)`).join('\n')}

    // Motion (ms)
${model.durations.map((d) => `    ${rp()} int ${camel(d.name)}Ms: ${fmtNum(d.ms)} // --${d.name}`).join('\n')}
}
`;
  return {
    relPath: 'qml/ThreadyTokens.qml',
    content,
    colors: modelColorSet(model),
    extractColors: (s) => new Set([...s.matchAll(/"#([0-9a-f]{6})"/g)].map((m) => up(m[1]))),
    symbolRe: /readonly property /g,
    symbols,
  };
}

/* 4.6 Lipgloss Go palette — matches lipgloss-theme.md §2/§3/§5/§6 EXACTLY */
const GO_NAME = {
  accent: 'Accent',
  'accent-on': 'AccentOn',
  fg: 'Fg',
  muted: 'Muted',
  border: 'BorderColor', // §3: avoids clashing with a Border style
  'border-strong': 'BorderHard', // §3 name
  success: 'Success',
  warn: 'Warn',
  danger: 'DangerColor', // §3 name
  brand: 'Brand',
  'brand-2': 'Brand2',
  bg: 'Bg',
  'surface-warm': 'SurfaceWarm',
};
const goShort = (n) => n.replace(/Color$/, ''); // DangerColor → DangerC / BorderColor → BorderC (doc §5 naming)

function emitGo(model, lipRows) {
  let symbols = 0;
  const v = () => (symbols += 1, '');
  const lightOf = (token) => model.colors.find((c) => c.name === token).light;
  const trueVars = lipRows
    .map((r) => {
      v();
      return `\t// --${r.token} (dark) — ${r.role} [VERIFIED — tokens.css]\n\t${GO_NAME[r.token]} = lipgloss.Color("#${up(r.hex)}")`;
    })
    .join('\n');
  const completeVars = lipRows
    .map((r) => {
      v();
      return (
        `\t// --${r.token} → ANSI-256 ${r.ansi256}, ANSI-16 ${r.ansi16}${r.ansi16Name ? ' ' + r.ansi16Name : ''} — ANSI picks ${r.mapping}\n` +
        `\t${goShort(GO_NAME[r.token])}C = lipgloss.CompleteColor{TrueColor: "#${up(r.hex)}", ANSI256: "${r.ansi256}", ANSI: "${r.ansi16}"}`
      );
    })
    .join('\n');
  const adaptiveVars = lipRows
    .map((r) => {
      v();
      return (
        `\t// --${r.token} light/dark — light value VERIFIED (tokens.css), pairing ASSUMED (§6)\n` +
        `\t${goShort(GO_NAME[r.token])}A = lipgloss.AdaptiveColor{Light: "#${up(lightOf(r.token))}", Dark: "#${up(r.hex)}"}`
      );
    })
    .join('\n');
  const content = `// Code generated by tokens-bridge/generate.mjs from ../opendesign/tokens.css. DO NOT EDIT.
${header(model, '//', [
    'Normative mapping: ../screens/tui/lipgloss-theme.md §2 (truecolor → ANSI-256 → ANSI-16);',
    'the generator FAILS if that table drifts from tokens.css. All ANSI-256/ANSI-16 indices',
    'are ASSUMED nearest-color picks per lipgloss-theme.md §1/§2 — re-verify on real',
    '16-color terminals [OPEN: THREADY-DES-17].',
  ])}

// Package theme is the generated Thready Lipgloss palette. The TUI defaults to
// the terminal's dark surface (design-system.md §7), so the plain vars carry
// the DARK theme values.
package theme

import "github.com/charmbracelet/lipgloss"

// Truecolor palette — dark theme values (lipgloss-theme.md §3 names).
var (
${trueVars}
)

// Pinned degradation (lipgloss-theme.md §5, option 2 — recommended for the
// 16-color floor): every terminal profile fixed explicitly. ANSI picks ASSUMED.
var (
${completeVars}
)

// Adaptive light/dark pairing (lipgloss-theme.md §6) — ASSUMED option; the TUI
// ships dark-only today. Adoption folds into [OPEN: THREADY-DES-17].
var (
${adaptiveVars}
)
`;
  return {
    relPath: 'lipgloss/thready_palette.go',
    content,
    colors: (() => {
      const s = new Set();
      for (const r of lipRows) {
        s.add(up(r.hex));
        s.add(up(lightOf(r.token)));
      }
      return s;
    })(),
    extractColors: (s) => new Set([...s.matchAll(/"#([0-9A-F]{6})"/g)].map((m) => up(m[1]))),
    symbolRe: /^\t[A-Z][A-Za-z0-9]* += /gm,
    symbols,
  };
}

/* 4.7 Flutter (helix_design Flutter arm — verified empty scaffold, GAP: 8.2/8.3) */
function emitFlutter(model) {
  let symbols = 0;
  const sc = () => (symbols += 1, 'static const');
  const colorClass = (cls, mode) => {
    const lines = [`abstract final class ${cls} {`];
    for (const c of model.colors) {
      lines.push(`  ${sc()} Color ${camel(c.name)} = Color(0xFF${up(c[mode])}); // --${c.name} (${mode})`);
    }
    for (const a of model.aliases) {
      lines.push(`  ${sc()} Color ${camel(a.name)} = ${camel(a.target)}; // --${a.name} → var(--${a.target})`);
    }
    lines.push('}');
    return lines.join('\n');
  };
  const spacing = model.dims.filter((d) => d.name.startsWith('space-') || d.name.startsWith('section-') || d.name.startsWith('container-'));
  const radius = model.dims.filter((d) => d.name.startsWith('radius-'));
  const type = model.dims.filter((d) => d.name.startsWith('text-'));
  const content = `${header(model, '//', [
    'Delivery-ready contract only: the Flutter arm of helix_design is a verified empty',
    'scaffold [GAP: 8.2/8.3] — nothing consumes this yet (library/platform-map.md §2).',
  ])}

import 'dart:ui' show Color;

const String threadyThemeId = '${model.themeId}'; // --theme-id

${colorClass('ThreadyColorsLight', 'light')}

${colorClass('ThreadyColorsDark', 'dark')}

abstract final class ThreadySpacing { // logical px
${spacing.map((d) => `  ${sc()} double ${spaceShort(d.name)} = ${fmtNum(d.px)}; // --${d.name}`).join('\n')}
}

abstract final class ThreadyRadius { // logical px
${radius.map((d) => `  ${sc()} double ${radiusShort(d.name)} = ${fmtNum(d.px)}; // --${d.name}`).join('\n')}
}

abstract final class ThreadyTypeScale { // logical px
${type.map((d) => `  ${sc()} double ${camel(d.name)} = ${fmtNum(d.px)}; // --${d.name}`).join('\n')}
${model.numbers.map((n) => `  ${sc()} double ${camel(n.name)} = ${n.value}; // --${n.name} (line-height multiplier)`).join('\n')}
${model.tracking.map((t) => `  ${sc()} double ${camel(t.name)} = ${fmtNum(t.em)}; // --${t.name} (em)`).join('\n')}
}

abstract final class ThreadyMotion { // milliseconds
${model.durations.map((d) => `  ${sc()} int ${camel(d.name)}Ms = ${fmtNum(d.ms)}; // --${d.name}`).join('\n')}
}
`;
  return {
    relPath: 'flutter/thready_tokens.dart',
    content,
    colors: modelColorSet(model),
    extractColors: (s) => new Set([...s.matchAll(/Color\(0xFF([0-9A-F]{6})\)/g)].map((m) => up(m[1]))),
    symbolRe: /static const /g,
    symbols,
  };
}

/* ────────────────────────────────────────────────────────────────────────────
 * 5. Cross-checks (used by --check; also usable after generation)
 * ──────────────────────────────────────────────────────────────────────────── */

function setEq(a, b) {
  if (a.size !== b.size) return false;
  for (const x of a) if (!b.has(x)) return false;
  return true;
}

function balancedDelims(s) {
  const pairs = { '{': '}', '(': ')', '[': ']' };
  const counts = {};
  for (const ch of s) {
    if ('{}()[]'.includes(ch)) counts[ch] = (counts[ch] || 0) + 1;
  }
  return Object.entries(pairs).every(([o, c]) => (counts[o] || 0) === (counts[c] || 0));
}

function runChecks(targets, model) {
  const results = [];
  const ok = (name, pass, detail = '') => results.push({ name, pass, detail });

  for (const t of targets) {
    const c = t.content;
    // (a) hex round-trip: every color literal in the output maps back to tokens.css
    const found = t.extractColors(c);
    ok(
      `${t.relPath}: hex round-trip vs tokens.css`,
      setEq(found, t.colors),
      `emitted ${found.size} unique hexes, expected ${t.colors.size}`,
    );
    // (b) structural: balanced delimiters
    ok(`${t.relPath}: balanced {} () []`, balancedDelims(c));
    // (c) structural: expected symbol count
    const n = (c.match(t.symbolRe) || []).length;
    ok(`${t.relPath}: symbol count`, n === t.symbols, `found ${n}, expected ${t.symbols}`);
  }
  // (d) both JSON targets parse as JSON
  for (const rel of ['web/tokens.json', 'penpot/tokens.penpot-import.json']) {
    const json = targets.find((t) => t.relPath === rel);
    try {
      JSON.parse(json.content);
      ok(`${rel}: JSON.parse`, true);
    } catch (e) {
      ok(`${rel}: JSON.parse`, false, String(e));
    }
  }
  // (d2) PenPot 2.17 quirk guards + structural diff vs the proven import file
  const penpot = targets.find((t) => t.relPath === 'penpot/tokens.penpot-import.json');
  try {
    const doc = JSON.parse(penpot.content);
    ok(
      'penpot/tokens.penpot-import.json: no top-level $description (PenPot 2.17 single-set quirk [VERIFIED 2026-07-22])',
      !('$description' in doc),
    );
    ok(
      'penpot/tokens.penpot-import.json: $themes (2) + $metadata.tokenSetOrder present',
      Array.isArray(doc.$themes) && doc.$themes.length === 2 &&
        Array.isArray(doc.$metadata?.tokenSetOrder) && doc.$metadata.tokenSetOrder.length === 3,
    );
  } catch (e) {
    ok('penpot/tokens.penpot-import.json: quirk guards', false, String(e));
  }
  if (existsSync(PENPOT_REFERENCE_PATH)) {
    let pass = false;
    let detail = '';
    try {
      const refRaw = readFileSync(PENPOT_REFERENCE_PATH, 'utf8');
      pass = JSON.stringify(JSON.parse(refRaw)) === JSON.stringify(JSON.parse(penpot.content));
      detail = !pass
        ? 'parsed-JSON mismatch vs exports/penpot/tokens.penpot-import.json'
        : refRaw === penpot.content
          ? 'byte-identical'
          : 'parsed-JSON equal (serialization differs)';
    } catch (e) {
      detail = String(e);
    }
    ok('penpot/tokens.penpot-import.json: structural diff vs proven 2026-07-22 import reference', pass, detail);
  } else {
    ok(
      'penpot/tokens.penpot-import.json: structural diff vs import reference',
      true,
      'reference exports/penpot/tokens.penpot-import.json absent — diff skipped',
    );
  }
  // (e) tokens.css internal consistency + lipgloss doc agreement happen during
  //     model/doc construction (throw on failure) — record that they held.
  ok('tokens.css: @media-dark == [data-theme=dark]/.dark blocks', true, 'validated during parse');
  ok('lipgloss-theme.md §2 truecolor == tokens.css dark values', true, 'validated during parse');
  ok(`tokens.css model: ${model.colors.length} colors ×2 modes, ${model.aliases.length} aliases, ${model.dims.length} px dims`, true);
  return results;
}

/* ────────────────────────────────────────────────────────────────────────────
 * 6. Main
 * ──────────────────────────────────────────────────────────────────────────── */

function emitAll() {
  const model = buildModel();
  const lipRows = parseLipglossDoc(model);
  const targets = [
    emitDtcg(model),
    emitPenpot(model),
    emitCompose(model),
    emitSwift(model),
    emitArkts(model),
    emitQml(model),
    emitGo(model, lipRows),
    emitFlutter(model),
  ];
  return { model, targets };
}

function writeTargets(root, targets) {
  for (const t of targets) {
    const p = join(root, t.relPath);
    mkdirSync(dirname(p), { recursive: true });
    // Atomic: write a sibling .tmp then rename — a concurrent stager/reader can
    // never observe a half-written target.
    writeFileSync(p + '.tmp', t.content);
    renameSync(p + '.tmp', p);
  }
}

const args = process.argv.slice(2);
const { model, targets } = emitAll();

// penpot-unsupported note (README §9): these tokens ARE included in
// penpot/tokens.penpot-import.json (mirroring the proven import file) but
// PenPot 2.17 silently drops them on import — 71/74 land.
const ppUnsupported = penpotUnsupported(model);
console.log(
  `penpot-unsupported: PenPot 2.17 silently drops ${ppUnsupported.length} token(s) on import ` +
  `[VERIFIED 2026-07-22]: ${ppUnsupported.join(', ')} — included in ` +
  'generated/penpot/tokens.penpot-import.json regardless (README §9).',
);

if (args.includes('--check')) {
  let failures = 0;
  const tmp = mkdtempSync(join(tmpdir(), 'thready-tokens-bridge-'));
  writeTargets(tmp, targets);
  // Drift gate: committed outputs must be byte-identical to a fresh generation.
  for (const t of targets) {
    const committedPath = join(OUT_ROOT, t.relPath);
    let status, detail = '';
    if (!existsSync(committedPath)) {
      status = false;
      detail = 'committed file missing — run: node generate.mjs';
    } else {
      const committed = readFileSync(committedPath, 'utf8');
      status = committed === t.content;
      if (!status) detail = `drift vs ${tmp}${sep}${t.relPath} — run: node generate.mjs`;
    }
    if (!status) failures += 1;
    console.log(`${status ? 'PASS' : 'FAIL'}  drift  generated/${t.relPath}${detail ? '  (' + detail + ')' : ''}`);
  }
  for (const r of runChecks(targets, model)) {
    if (!r.pass) failures += 1;
    console.log(`${r.pass ? 'PASS' : 'FAIL'}  check  ${r.name}${r.detail ? '  (' + r.detail + ')' : ''}`);
  }
  console.log(`\nsha256(tokens.css) = ${model.hash}`);
  console.log(failures === 0 ? 'CHECK OK — no drift, all cross-checks passed.' : `CHECK FAILED — ${failures} failure(s).`);
  process.exit(failures === 0 ? 0 : 1);
} else {
  writeTargets(OUT_ROOT, targets);
  const checks = runChecks(targets, model);
  const bad = checks.filter((r) => !r.pass);
  for (const t of targets) {
    console.log(`wrote generated/${t.relPath}  (${Buffer.byteLength(t.content)} bytes, ${t.symbols} tokens/symbols)`);
  }
  for (const r of bad) console.log(`FAIL  ${r.name}  (${r.detail})`);
  console.log(`sha256(tokens.css) = ${model.hash}`);
  if (bad.length) process.exit(1);
  console.log('GENERATE OK — self-checks passed.');
}
