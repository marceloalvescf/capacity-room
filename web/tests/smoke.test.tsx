import { render } from "@testing-library/react";
import { expect, it } from "vitest";
import { App } from "@/app/app";

it("renders the app without throwing", () => {
  const { container } = render(<App />);
  expect(container).toBeEmptyDOMElement();
});
