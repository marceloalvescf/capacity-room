import { describe, expect, it } from "vitest";
import en from "@/locales/en.json";
import ptBR from "@/locales/pt-BR.json";
import { findBannedTerms } from "./vocabulary";

function leaves(tree: object): string[] {
  return Object.values(tree).flatMap((value) => (typeof value === "string" ? [value] : leaves(value as object)));
}

describe("vocabulary", () => {
  it("no banned term in a locale", () => {
    for (const [lang, file] of [
      ["en", en],
      ["pt-BR", ptBR],
    ] as const) {
      const hits = leaves(file).flatMap((text) => findBannedTerms(lang, text).map((term) => `${text}: ${term}`));
      expect({ lang, hits }).toEqual({ lang, hits: [] });
    }
  });

  it("the finder", () => {
    expect(findBannedTerms("pt-BR", "o limite de CPU")).toEqual(["limite"]);
    expect(findBannedTerms("en", "cpu")).toEqual(["cpu"]);
    expect(findBannedTerms("en", "node group")).toEqual(["node group"]);
    expect(findBannedTerms("pt-BR", "{{limite}} ok")).toEqual([]);
  });
});
