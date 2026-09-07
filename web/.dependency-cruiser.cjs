module.exports = {
  forbidden: [
    { name: "no-cycles", severity: "error", from: {}, to: { circular: true } },
    { name: "no-unresolved", severity: "error", from: {}, to: { couldNotResolve: true } },
    {
      name: "no-production-to-tests",
      severity: "error",
      from: { path: "^src/", pathNot: "\\.(test|fixture)\\.tsx?$" },
      to: { path: "\\.(test|fixture)\\.tsx?$" },
    },
    {
      name: "lib-does-not-depend-on-ui-or-store",
      severity: "error",
      from: { path: "^src/lib/" },
      to: { path: "^src/(components/|store/|queries/|App\\.tsx$|main\\.tsx$)" },
    },
    {
      name: "store-does-not-depend-on-ui-or-queries",
      severity: "error",
      from: { path: "^src/store/" },
      to: { path: "^src/(components/|queries/|App\\.tsx$|main\\.tsx$)" },
    },
    {
      name: "queries-do-not-depend-on-ui",
      severity: "error",
      from: { path: "^src/queries/" },
      to: { path: "^src/(components/|App\\.tsx$|main\\.tsx$)" },
    },
    {
      name: "ui-primitives-only-use-ui-and-utils",
      severity: "error",
      from: { path: "^src/components/ui/" },
      to: { path: "^src/", pathNot: "^src/(components/ui/|lib/utils\\.ts$)" },
    },
    {
      name: "components-do-not-import-entrypoints",
      severity: "error",
      from: { path: "^src/components/" },
      to: { path: "^src/(App|main)\\.tsx$" },
    },
    {
      name: "schema-and-utils-stay-independent",
      severity: "error",
      from: { path: "^src/lib/(schema|utils)\\.ts$" },
      to: { path: "^src/", pathNot: "^src/lib/(schema|utils)\\.ts$" },
    },
  ],
  options: {
    doNotFollow: { path: "node_modules" },
    tsConfig: { fileName: "tsconfig.json" },
    // Include type-only edges so shared types cannot silently reverse a boundary.
    tsPreCompilationDeps: true,
    enhancedResolveOptions: {
      exportsFields: ["exports"],
      extensions: [".ts", ".tsx", ".d.ts", ".js", ".json"],
      conditionNames: ["types", "import", "browser", "default"],
    },
  },
}
