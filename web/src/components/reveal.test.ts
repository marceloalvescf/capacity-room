import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { revealRows } from "./reveal";

function box(top: number, bottom: number): HTMLElement {
  const element = document.createElement("tr");
  element.getBoundingClientRect = () => ({ top, bottom, height: bottom - top }) as DOMRect;
  return element;
}

let scrollBy: ReturnType<typeof vi.fn>;
let reduced: boolean;

beforeEach(() => {
  reduced = false;
  scrollBy = vi.fn();
  vi.stubGlobal("innerHeight", 768);
  vi.stubGlobal("scrollBy", scrollBy);
  vi.stubGlobal("matchMedia", (query: string) => ({ matches: query.includes("no-preference") ? !reduced : false }));
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("reveal rows", () => {
  it("scrolls down", () => {
    revealRows(box(600, 650), box(650, 1000), 40);
    expect(scrollBy).toHaveBeenCalledWith({ top: 240, behavior: "smooth" });
  });

  it("keeps the row's top", () => {
    revealRows(box(300, 350), box(350, 1400), 40);
    expect(scrollBy).toHaveBeenCalledWith({ top: 252, behavior: "smooth" });
  });

  it("scrolls up", () => {
    revealRows(box(20, 60), box(60, 300), 40);
    expect(scrollBy).toHaveBeenCalledWith({ top: -28, behavior: "smooth" });
  });

  it("stays", () => {
    revealRows(box(100, 150), box(150, 500), 40);
    expect(scrollBy).not.toHaveBeenCalled();
  });

  it("follows the motion setting", () => {
    reduced = true;
    revealRows(box(20, 60), box(60, 300), 40);
    expect(scrollBy).toHaveBeenLastCalledWith({ top: -28, behavior: "auto" });
    reduced = false;
    revealRows(box(20, 60), box(60, 300), 40);
    expect(scrollBy).toHaveBeenLastCalledWith({ top: -28, behavior: "smooth" });
  });
});
