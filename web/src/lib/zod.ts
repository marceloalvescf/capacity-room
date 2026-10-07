import { z } from "zod";

// Under the Content-Security-Policy, a compile probe is reported as a violation.
z.config({ jitless: true });
