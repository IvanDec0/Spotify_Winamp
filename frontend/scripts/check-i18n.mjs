#!/usr/bin/env node
/**
 * Translation guard: walks every Svelte template and reports literal,
 * user-facing copy that is not wrapped in `t()`.
 *
 * It checks text nodes and the `title`, `aria-label`, `placeholder` and `alt`
 * attributes. Brand names, numbers, units and separator symbols are allowed
 * so the compact player and skin chrome can keep their identity.
 *
 * Run with `npm run i18n:check`. No `--fix` mode: catalogs are edited by hand.
 */
import { readFileSync, readdirSync } from "node:fs";
import { join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import { parse } from "svelte/compiler";

const FRONTEND_ROOT = fileURLToPath(new URL("..", import.meta.url));
const SRC_DIR = join(FRONTEND_ROOT, "src");
const SKIP_DIRS = new Set(["node_modules", "bindings", "dist"]);
const CHECKED_ATTRIBUTES = new Set(["title", "aria-label", "placeholder", "alt"]);
const BRANDS = ["CADENCE", "Cadence", "Clarity", "Spotify", "Premium", "JAM"];
const SYMBOLS = ["%", "·", "—", "–", "×", "↗", "▶", "◀", "→", "←", "•", "+", "-"];
const UNITS = ["kbps", "kHz", "Hz", "GB", "MB", "ms", "min", "s"];
const SKIPPED_KEYS = new Set([
  "expression",
  "context",
  "index",
  "key",
  "test",
  "value",
  "loc",
  "name_loc",
  "metadata",
  "css",
  "js",
  "instance",
  "module",
  "options",
  "comments",
]);

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

const BRAND_PATTERNS = BRANDS.map((brand) => new RegExp(`\\b${escapeRegExp(brand)}\\b`, "gu"));
const UNIT_PATTERNS = UNITS.map((unit) => new RegExp(`\\b${escapeRegExp(unit)}\\b`, "gu"));
const NUMBER_PATTERN = /\d+(?:[.,]\d+)*/gu;

/** True when nothing but brand names, numbers, units or symbols remains. */
function isAllowlisted(text) {
  let rest = text;
  for (const pattern of BRAND_PATTERNS) rest = rest.replace(pattern, " ");
  rest = rest.replace(NUMBER_PATTERN, " ");
  for (const symbol of SYMBOLS) rest = rest.split(symbol).join(" ");
  for (const pattern of UNIT_PATTERNS) rest = rest.replace(pattern, " ");
  return rest.replace(/\s/gu, "") === "";
}

function findSvelteFiles(dir, files = []) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    if (entry.isDirectory()) {
      if (!SKIP_DIRS.has(entry.name)) findSvelteFiles(join(dir, entry.name), files);
      continue;
    }
    if (entry.isFile() && entry.name.endsWith(".svelte")) {
      files.push(join(dir, entry.name));
    }
  }
  return files;
}

function lineAt(source, offset) {
  let line = 1;
  for (let index = 0; index < offset && index < source.length; index += 1) {
    if (source[index] === "\n") line += 1;
  }
  return line;
}

function summarize(text) {
  const flat = text.replace(/\s+/gu, " ").trim();
  return flat.length > 80 ? `${flat.slice(0, 77)}...` : flat;
}

/** Reads literal text out of an attribute value, if it has any. */
function literalAttributeText(attribute) {
  const parts = Array.isArray(attribute.value) ? attribute.value : [attribute.value];
  const texts = [];
  for (const part of parts) {
    if (part?.type === "Text" && part.data.trim()) texts.push(part);
  }
  if (texts.length === 0) return null;
  return {
    text: texts
      .map((part) => part.data)
      .join(" ")
      .trim(),
    offset: texts[0].start,
  };
}

function walk(node, visit) {
  if (Array.isArray(node)) {
    for (const child of node) walk(child, visit);
    return;
  }
  if (!node || typeof node !== "object") return;
  if (typeof node.type === "string") visit(node);
  for (const [key, value] of Object.entries(node)) {
    if (SKIPPED_KEYS.has(key)) continue;
    walk(value, visit);
  }
}

function checkFile(file) {
  const source = readFileSync(file, "utf8");
  const findings = [];
  let ast;
  try {
    ast = parse(source, { modern: true });
  } catch (error) {
    const reason = error instanceof Error ? error.message : String(error);
    return [{ line: 1, message: `could not parse the template: ${reason}` }];
  }

  walk(ast.fragment, (node) => {
    if (node.type === "Text") {
      const text = node.data.trim();
      if (text && !isAllowlisted(text)) {
        findings.push({
          line: lineAt(source, node.start),
          message: `literal text "${summarize(node.data)}" is not translated with t()`,
        });
      }
      return;
    }
    if (node.type !== "Attribute" || !CHECKED_ATTRIBUTES.has(node.name)) return;
    const literal = literalAttributeText(node);
    if (literal && !isAllowlisted(literal.text)) {
      findings.push({
        line: lineAt(source, literal.offset),
        message: `literal ${node.name} "${summarize(literal.text)}" is not translated with t()`,
      });
    }
  });

  return findings;
}

const files = findSvelteFiles(SRC_DIR).sort();
const reports = [];
for (const file of files) {
  const path = relative(FRONTEND_ROOT, file);
  for (const finding of checkFile(file)) {
    reports.push({ path, ...finding });
  }
}

reports.sort((left, right) => left.path.localeCompare(right.path) || left.line - right.line);

for (const report of reports) {
  console.log(`${report.path}:${report.line}: ${report.message}`);
}

process.exitCode = reports.length > 0 ? 1 : 0;
