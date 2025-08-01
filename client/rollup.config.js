import resolve from "@rollup/plugin-node-resolve";
import typescript from "@rollup/plugin-typescript";

export default [
  // UMD build
  {
    input: "src/index.ts",
    output: {
      file: "dist/index.js",
      format: "umd",
      name: "StateTemplateClient",
      sourcemap: true,
    },
    plugins: [
      resolve(),
      typescript({
        tsconfig: "./tsconfig.build.json",
        declaration: true,
        declarationDir: "./dist",
      }),
    ],
  },
  // ESM build
  {
    input: "src/index.ts",
    output: {
      file: "dist/index.esm.js",
      format: "esm",
      sourcemap: true,
    },
    plugins: [
      resolve(),
      typescript({
        tsconfig: "./tsconfig.build.json",
      }),
    ],
  },
];
