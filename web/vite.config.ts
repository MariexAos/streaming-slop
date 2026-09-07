import tailwindcss from "@tailwindcss/vite"
import react from "@vitejs/plugin-react"
import reactHooks from "eslint-plugin-react-hooks"
import { defineConfig } from "vite-plus"

const hookRules = Object.fromEntries(
  Object.entries(reactHooks.configs.recommended.rules)
    .filter(
      ([name]) => !["react-hooks/rules-of-hooks", "react-hooks/exhaustive-deps"].includes(name),
    )
    .map(([name]) => [name.replace("react-hooks/", "react-hooks-js/"), "error"]),
)

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: { alias: { "@": new URL("./src", import.meta.url).pathname } },
  build: { outDir: "../internal/adapter/in/webui/dist", emptyOutDir: true },
  fmt: {
    semi: false,
    singleQuote: false,
    ignorePatterns: ["pnpm-lock.yaml", "test-results/**", "playwright-report/**"],
  },
  lint: {
    plugins: ["typescript", "react", "oxc"],
    jsPlugins: [{ name: "react-hooks-js", specifier: "eslint-plugin-react-hooks" }],
    options: { typeAware: true, typeCheck: true },
    categories: { correctness: "error" },
    ignorePatterns: ["test-results/**", "playwright-report/**"],
    rules: {
      ...hookRules,
      "react/rules-of-hooks": "error",
      "react/exhaustive-deps": "error",
      "react/only-export-components": ["error", { allowConstantExport: true }],
      "typescript/no-explicit-any": "error",
      "typescript/no-floating-promises": "error",
      "typescript/no-misused-promises": "error",
      "typescript/await-thenable": "error",
      "typescript/unbound-method": "error",
      "typescript/no-unsafe-assignment": "error",
      "typescript/no-unsafe-argument": "error",
      "typescript/no-unsafe-call": "error",
      "typescript/no-unsafe-member-access": "error",
      "typescript/no-unsafe-return": "error",
      "typescript/no-unnecessary-type-assertion": "error",
      "typescript/switch-exhaustiveness-check": "error",
      "max-lines": ["error", { max: 500, skipBlankLines: true, skipComments: true }],
    },
  },
  test: { environment: "node", include: ["src/**/*.test.ts"], allowOnly: false },
})
