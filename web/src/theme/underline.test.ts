/// <reference types="node" />
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { expect, it } from "vitest";

const css = readFileSync(join(import.meta.dirname, "theme.css"), "utf8");

// Mona Sans draws its underline on the baseline, where skipping the ink of the letters cuts it into dashes; the root
// sets every underline below the letters, in the base layer so a utility still wins.
it("the root sets every underline below the letters, in the base layer", () => {
  const base = css.slice(css.indexOf("@layer base {"));
  expect(base).toMatch(/^@layer base \{\s*\/\*[\s\S]*?\*\/\s*html \{\s*text-underline-offset: 0\.22em;\s*\}/);
});
