// Flat ESLint config (ESLint 9 / Next 16). `next lint` was removed in Next 16, so we
// run ESLint directly (see the "lint" script) using the flat configs that
// eslint-config-next ships: core-web-vitals (React + a11y + import + Next rules) and
// typescript (typescript-eslint rules). Both export flat-config arrays.
import coreWebVitals from "eslint-config-next/core-web-vitals";
import typescript from "eslint-config-next/typescript";

const eslintConfig = [
  {
    ignores: [
      ".next/**",
      ".next-edge/**",
      "out/**",
      "build/**",
      "dist/**",
      "node_modules/**",
      "next-env.d.ts",
      "public/**",
    ],
  },
  ...coreWebVitals,
  ...typescript,
  {
    // Calibration for an existing codebase. These are downgraded from error to
    // warning so `pnpm lint` stays green on long-established, intentional patterns
    // while still surfacing them — flip any back to "error" to burn the debt down:
    //   - no-explicit-any: `any` is used in several API-transform spots.
    //   - set-state-in-effect / immutability / purity: react-hooks v7's newer
    //     React-Compiler lint family; this app doesn't use the compiler and relies
    //     on the ubiquitous "fetch on mount" effect pattern.
    // rules-of-hooks, exhaustive-deps, and the rest stay at preset strength.
    rules: {
      "@typescript-eslint/no-explicit-any": "warn",
      "react-hooks/set-state-in-effect": "warn",
      "react-hooks/immutability": "warn",
      "react-hooks/purity": "warn",
    },
  },
];

export default eslintConfig;
