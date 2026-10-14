import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

// The report's zone, never the process's, decides every wall time; a process
// in UTC shows a lookup of the wrong zone.
process.env["TZ"] = "UTC";

afterEach(cleanup);

// jsdom lays nothing out and implements neither scrolling nor media queries; a row that opens would otherwise fail.
// A test that needs an answer stubs its own.
Object.defineProperty(window, "scrollBy", { configurable: true, writable: true, value: () => undefined });
Object.defineProperty(Element.prototype, "scrollIntoView", { configurable: true, writable: true, value: () => undefined });
Object.defineProperty(window, "matchMedia", {
  configurable: true,
  writable: true,
  value: (query: string) => ({ matches: false, media: query }),
});
