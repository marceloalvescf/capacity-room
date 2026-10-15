import { render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { i18next } from "@/i18n/i18n";
import { DeleteDialog, type DeleteTarget } from "./delete-dialog";

afterEach(() => {
  vi.restoreAllMocks();
});

// jsdom runs no animation, so Radix unmounts a closed layer at once. Naming an animation for each state, as
// theme.css does, makes Radix wait for an animation end that never comes: the closed dialog stays, as it does in a
// browser while it fades.
function holdClosedLayers() {
  const real = window.getComputedStyle.bind(window);
  vi.spyOn(window, "getComputedStyle").mockImplementation((element, pseudo) => {
    const style = real(element, pseudo);
    // Radix keeps the one style object it read at mount, so the name follows the element's state when read.
    const name = () => {
      const state = element.getAttribute("data-state");
      return state === "open" ? "dialog-in" : state === "closed" ? "dialog-out" : style.animationName;
    };
    return new Proxy(style, {
      get: (target, key) => {
        if (key === "animationName") {
          return name();
        }
        const value: unknown = Reflect.get(target, key);
        return typeof value === "function" ? (value as (...args: unknown[]) => unknown).bind(target) : value;
      },
    });
  });
}

const one: DeleteTarget = { kind: "one", id: "r1", profile: "Production", time: "2026-10-15 12:00", username: "ana" };

it("keeps its title and body while it fades out after closing", async () => {
  await i18next.changeLanguage("en");
  holdClosedLayers();
  const props = { busy: false, onConfirm: () => undefined, onCancel: () => undefined, onCloseAutoFocus: () => undefined };
  const view = render(<DeleteDialog target={one} {...props} />);
  const title = i18next.t("delete.oneTitle", { profile: "Production" });
  expect(screen.getByRole("alertdialog", { name: title })).toBeInTheDocument();

  view.rerender(<DeleteDialog target={null} {...props} />);
  const closing = screen.getByRole("alertdialog", { hidden: true });
  expect(closing).toHaveAttribute("data-state", "closed");
  expect(closing).toHaveAccessibleName(title);
  expect(closing).toHaveTextContent(i18next.t("delete.oneBody", { time: "2026-10-15 12:00", username: "ana" }));
});
