import { create } from "@bufbuild/protobuf";
import { describe, expect, it } from "vitest";

import {
  GeographyService,
  ListRegionsRequestSchema,
  PROTO_API_VERSION,
} from "../src/index";

describe("generated v1 contract", () => {
  it("exports buildable messages and the complete service descriptor", () => {
    const request = create(ListRegionsRequestSchema, { limit: 16 });

    expect(request.limit).toBe(16);
    expect(GeographyService.methods).toHaveLength(13);
    expect(GeographyService.methods.at(-1)?.methodKind).toBe("server_streaming");
    expect(PROTO_API_VERSION).toBe("ghanageo.v1");
  });
});
