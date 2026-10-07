import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

// The report's zone, never the process's, decides every wall time; a process
// in UTC shows a lookup of the wrong zone.
process.env["TZ"] = "UTC";

afterEach(cleanup);
