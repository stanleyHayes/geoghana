import type { NextConfig } from "next";

const config: NextConfig = {
  transpilePackages: ["@ghanageo/ui"],
  reactStrictMode: true,
  outputFileTracingIncludes: {
    "/api/grpc": ["../../proto/ghanageo/v1/geography.proto"],
  },
};
export default config;
