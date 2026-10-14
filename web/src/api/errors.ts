import type { TFunction } from "i18next";
import { ApiFailure } from "./client";

export function toError(thrown: unknown): Error {
  return thrown instanceof Error || thrown instanceof DOMException ? thrown : new Error("unexpected failure");
}

// A failed request is worded by its code; a request that never got an answer
// is a network failure, or a timeout when none came in time.
export function errorText(t: TFunction, failure: Error): string {
  if (!(failure instanceof ApiFailure)) {
    if (failure.name === "TimeoutError") {
      return t("error.timeout");
    }
    return t("error.network");
  }
  if (failure.code === null) {
    return t("error.general");
  }
  return failure.retryAfter === null
    ? t(`error.${failure.code}`)
    : t(`error.${failure.code}`, { seconds: failure.retryAfter });
}
