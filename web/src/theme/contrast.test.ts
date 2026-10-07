/// <reference types="node" />
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { pairs } from "./pairs";

const css = readFileSync(join(import.meta.dirname, "theme.css"), "utf8");

function tokens(selector: string): Map<string, string> {
  const start = css.indexOf(`${selector} {`);
  const end = css.indexOf("}", start);
  const found = new Map<string, string>();
  for (const m of css.slice(start, end).matchAll(/--([a-z-]+):\s*(#[0-9a-f]{6});/g)) {
    found.set(m[1] ?? "", m[2] ?? "");
  }
  return found;
}

const themes = {
  light: tokens(":root"),
  dark: tokens(':root[data-theme="dark"]'),
};

function channel(hex: string, at: number): number {
  const c = parseInt(hex.slice(at, at + 2), 16) / 255;
  return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
}

function luminance(hex: string): number {
  return 0.2126 * channel(hex, 1) + 0.7152 * channel(hex, 3) + 0.0722 * channel(hex, 5);
}

function ratio(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return ((hi ?? 0) + 0.05) / ((lo ?? 0) + 0.05);
}

describe("contrast", () => {
  it("every pair meets its minimum", () => {
    const failures: string[] = [];
    for (const [theme, map] of Object.entries(themes)) {
      for (const p of pairs) {
        const fg = map.get(p.fg);
        const bg = map.get(p.bg);
        if (fg === undefined || bg === undefined) {
          failures.push(`${theme}: ${p.fg} on ${p.bg} names a missing token`);
          continue;
        }
        const r = ratio(fg, bg);
        if (r < p.min) {
          failures.push(`${theme}: ${p.fg} on ${p.bg} is ${r.toFixed(2)}, needs ${String(p.min)}`);
        }
      }
    }
    expect(failures).toEqual([]);
  });

  it("every token has a dark twin", () => {
    expect(themes.light.size).toBe(22);
    expect([...themes.dark.keys()].sort()).toEqual([...themes.light.keys()].sort());
  });
});
