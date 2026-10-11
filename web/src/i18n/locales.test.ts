import { describe, expect, it } from "vitest";
import indexHtml from "../../index.html?raw";
import en from "@/locales/en.json";
import ptBR from "@/locales/pt-BR.json";
import { families } from "./families";

function flatten(tree: object, prefix = ""): Map<string, string> {
  const out = new Map<string, string>();
  for (const [key, value] of Object.entries(tree)) {
    const path = prefix === "" ? key : `${prefix}.${key}`;
    if (typeof value === "string") {
      out.set(path, value);
    } else {
      for (const [k, v] of flatten(value as object, path)) {
        out.set(k, v);
      }
    }
  }
  return out;
}

function params(text: string): string[] {
  return [...text.matchAll(/\{\{(.*?)\}\}/g)].map((m) => m[1] ?? "").sort();
}

const sources = Object.entries(
  import.meta.glob<string>(["/src/**/*.ts", "/src/**/*.tsx", "!/src/**/*.test.*"], {
    query: "?raw",
    import: "default",
    eager: true,
  }),
).map(([, text]) => text);

function isUsed(key: string): boolean {
  const family = Object.keys(families).find((name) => key.startsWith(`${name}.`));
  if (family !== undefined) {
    return true;
  }
  return sources.some((text) => [`"${key}"`, `'${key}'`, `\`${key}\``].some((literal) => text.includes(literal)));
}

const files = { en: flatten(en), "pt-BR": flatten(ptBR) };

describe("locales", () => {
  it("same keys, same parameters", () => {
    expect([...files["pt-BR"].keys()].sort()).toEqual([...files.en.keys()].sort());
    for (const [key, text] of files.en) {
      expect({ key, params: params(files["pt-BR"].get(key) ?? "") }).toEqual({ key, params: params(text) });
    }
  });

  it("families are exact", () => {
    for (const [family, members] of Object.entries(families)) {
      for (const [lang, file] of Object.entries(files)) {
        const keys = [...file.keys()]
          .filter((key) => key.startsWith(`${family}.`))
          .map((key) => key.slice(family.length + 1))
          .sort();
        expect({ family, lang, keys }).toEqual({ family, lang, keys: [...members].sort() });
      }
    }
  });

  it("product name", () => {
    const page = /<title>(.*)<\/title>/.exec(indexHtml)?.[1];
    for (const [lang, file] of Object.entries(files)) {
      const name = file.get("app.name");
      expect({ lang, short: file.get("app.short"), page, title: file.get("app.title") }).toEqual({
        lang,
        short: name,
        page: name,
        title: `{{screen}} · ${name ?? ""}`,
      });
    }
  });

  it("every key is used", () => {
    const unused = [...files.en.keys()].filter((key) => !isUsed(key));
    expect(unused).toEqual([]);
  });
});
