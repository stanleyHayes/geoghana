# `@ghanageo/proto`

Generated TypeScript message and service descriptors for GhanaGeo's public
`ghanageo.v1.GeographyService`, plus the versioned source `.proto` file.

```ts
import { create } from "@bufbuild/protobuf";
import { GeographyService, SearchRequestSchema } from "@ghanageo/proto";

const request = create(SearchRequestSchema, { query: "Osu", limit: 10 });
```

Use `GeographyService` with a protobuf/connect transport of your choice. The
contract source is also exported as `@ghanageo/proto/geography.proto` for
non-TypeScript code generation. Regenerate with `pnpm generate`; generated
files must remain in sync with `proto/ghanageo/v1/geography.proto`.
