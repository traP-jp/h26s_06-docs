import cssPlugin from "@eslint/css";
import js from "@eslint/js";
import eslintConfigPrettier from "eslint-config-prettier";
import eslintPluginJsonc from "eslint-plugin-jsonc";
import eslintPluginUnicorn from "eslint-plugin-unicorn";
import globals from "globals";
import tseslint from "typescript-eslint";

export default tseslint.config(
    {
        // Global ignores
        ignores: [
            "**/dist/**",
            "**/node_modules/**",
            "**/.tsbuildinfo",
            "**/generated/**",
            "**/prisma/migrations/**",
            "**/vite.config.ts.timestamp-*",
        ],
    },
    js.configs.recommended,
    ...tseslint.configs.recommended,
    {
        files: ["**/*.{ts,tsx}"],
        languageOptions: {
            parser: tseslint.parser,
            parserOptions: {
                ecmaVersion: "latest",
                sourceType: "module",
                tsconfigRootDir: import.meta.dirname,
            },
            globals: {
                ...globals.browser,
                ...globals.node,
            },
        },
        plugins: {
            js,
            unicorn: eslintPluginUnicorn,
        },
        rules: {
            "unicorn/no-array-reduce": "off",
            "unicorn/filename-case": [
                "error",
                {
                    cases: {
                        pascalCase: true,
                        camelCase: true,
                    },
                    ignore: [String.raw`vite-env\.d\.ts`],
                },
            ],
            "unicorn/no-null": "off",
            "unicorn/prevent-abbreviations": [
                "error",
                {
                    replacements: {
                        env: false,
                        "vite-env": false,
                        args: false,
                        res: false,
                        props: false,
                        Props: false,
                        params: false,
                    },
                },
            ],
            "unicorn/consistent-compound-words": [
                "error",
                {
                    replacements: {
                        userName: false,
                    },
                },
            ],
            "@typescript-eslint/no-unused-vars": [
                "warn",
                {
                    argsIgnorePattern: "^_",
                    varsIgnorePattern: "^_",
                },
            ],
        },
    },
    // CSS config
    {
        files: ["**/*.css"],
        language: "css/css",
        ...cssPlugin.configs.recommended,
        rules: {
            "no-irregular-whitespace": "off",
        },
    },
    // JSON config
    {
        files: ["**/*.json", "**/*.jsonc"],
        plugins: {
            jsonc: eslintPluginJsonc,
        },
    },
    ...eslintPluginJsonc.configs["flat/prettier"],
    {
        files: ["**/*.test.{ts,tsx}"],
        rules: {
            "@typescript-eslint/no-explicit-any": "off",
            "unicorn/prevent-abbreviations": "off",
            "@typescript-eslint/no-unused-vars": "off",
        },
    },
    eslintConfigPrettier
);
