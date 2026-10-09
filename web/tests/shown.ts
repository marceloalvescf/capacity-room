import { screen, waitFor } from "@testing-library/react";
import { expect } from "vitest";

// The elements that hold a text on the page, leaving out the copy the live region keeps.
export function shown(text: string): HTMLElement[] {
  return screen.queryAllByText(text).filter((element) => element.closest("[role=status]") === null);
}

// The one element that holds a text on the page, once it appears.
export async function findShown(text: string): Promise<HTMLElement> {
  await waitFor(() => {
    expect(shown(text)).toHaveLength(1);
  });
  return shown(text)[0] as HTMLElement;
}
