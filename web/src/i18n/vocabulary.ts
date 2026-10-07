import banned from "../../../internal/catalog/banned-terms.json";

type Lang = keyof typeof banned;

const paramPattern = /\{\{.*?\}\}/gu;
const wordPattern = /[\p{L}\p{N}_-]+/gu;

// findBannedTerms returns every banned term found in text, once each, in the
// order first found. It reads the same file as the catalog guard in Go.
export function findBannedTerms(lang: Lang, text: string): string[] {
  const terms = banned[lang];
  const anyCase = new Set<string>(terms.any_case);
  const asWritten = new Set<string>(terms.as_written);
  const words = text.replace(paramPattern, " ").match(wordPattern) ?? [];
  const lower = words.map((word) => word.toLowerCase());

  const found = new Set<string>();
  lower.forEach((word, i) => {
    if (anyCase.has(word)) {
      found.add(word);
    }
    const written = words[i] ?? "";
    if (asWritten.has(written)) {
      found.add(written);
    }
  });

  for (const phrase of terms.phrases) {
    for (let i = 0; i + phrase.length <= lower.length; i++) {
      if (phrase.every((word, j) => lower[i + j] === word)) {
        found.add(phrase.join(" "));
      }
    }
  }
  return [...found];
}
