import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    exclude: ["bin/**", "dist/**", "node_modules/**"],
  },
});
