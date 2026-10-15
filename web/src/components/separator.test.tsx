/// <reference types="node" />
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { render } from "@testing-library/react";
import { expect, it } from "vitest";
import { Separator } from "./separator";

const root = join(import.meta.dirname, "..");

function components(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) {
      return components(path);
    }
    return entry.name.endsWith(".tsx") && !entry.name.endsWith(".test.tsx") ? [path] : [];
  });
}

it("parts two items by a gap and keeps the middle dot for screen readers alone", () => {
  const { container } = render(
    <p>
      <span>{"a"}</span>
      <Separator />
      <span>{"b"}</span>
    </p>,
  );
  const line = container.querySelector("p");
  expect(line?.textContent).toBe("a · b");
  expect(line?.querySelector(".sr-only")?.textContent).toBe(" ·");
  expect(line?.querySelector('[aria-hidden="true"]')).toHaveClass("me-4");
});

it("no component parts the items of a line with a drawn middle dot", () => {
  const found = components(root).flatMap((path) =>
    readFileSync(path, "utf8")
      .split("\n")
      .flatMap((line, index) =>
        /\{" · "\}|\? " · " :|^\s*·\s*$|>\s*·\s*</.test(line) ? [`${path.slice(root.length + 1)}:${String(index + 1)}`] : [],
      ),
  );
  expect(found).toEqual([]);
});
