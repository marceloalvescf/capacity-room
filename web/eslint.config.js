import js from "@eslint/js";
import vitest from "@vitest/eslint-plugin";
import jsxA11y from "eslint-plugin-jsx-a11y";
import playwright from "eslint-plugin-playwright";
import reactHooks from "eslint-plugin-react-hooks";
import { defineConfig } from "eslint/config";
import globals from "globals";
import tseslint from "typescript-eslint";

const literalText = {
  selector: "JSXText[value=/\\p{L}/u]",
  message: "Words on screen come from the locale files",
};

const restrictedSyntax = [
  "error",
  {
    selector: "JSXAttribute[name.name=/^(style|dangerouslySetInnerHTML)$/]",
    message: "Styles come from Tailwind classes, and markup is never injected",
  },
  {
    selector:
      "CallExpression[callee.type='MemberExpression'][callee.property.name=/^(skip|only|todo|fixme)$/][callee.object.name=/^(it|test|describe)$/]",
    message: "A test is neither skipped, focused, nor left as a todo",
  },
  literalText,
  {
    selector:
      "CallExpression[callee.object.name='document'][callee.property.name='createElement'][arguments.0.value='style']",
    message: "A style element is never created at run time",
  },
];

export default defineConfig(
  { ignores: ["dist", "node_modules", "test-results", "playwright-report", "src/api/schema.gen.ts"] },
  js.configs.recommended,
  tseslint.configs.strictTypeChecked,
  reactHooks.configs.flat.recommended,
  jsxA11y.flatConfigs.strict,
  {
    languageOptions: {
      globals: { ...globals.browser },
      parserOptions: {
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
      },
    },
    rules: { "no-restricted-syntax": restrictedSyntax },
  },
  {
    files: ["*.config.js", "*.config.ts"],
    extends: [tseslint.configs.disableTypeChecked],
    languageOptions: { globals: { ...globals.node } },
  },
  {
    files: ["public/**/*.js", "e2e/**/*.js"],
    extends: [tseslint.configs.disableTypeChecked],
    languageOptions: { sourceType: "script" },
  },
  {
    files: ["**/*.test.ts", "**/*.test.tsx", "tests/**"],
    plugins: { vitest },
    rules: {
      "vitest/no-disabled-tests": "error",
      "vitest/no-focused-tests": "error",
    },
  },
  {
    files: ["e2e/**"],
    plugins: { playwright },
    rules: {
      "playwright/no-skipped-test": "error",
      "playwright/no-focused-test": "error",
    },
  },
  {
    files: ["tests/**", "e2e/**"],
    rules: {
      "no-restricted-syntax": ["error", ...restrictedSyntax.slice(1).filter((r) => r !== literalText)],
    },
  },
);
