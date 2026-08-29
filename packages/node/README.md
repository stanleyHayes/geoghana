# @ghanageo/node

Server-only helpers for GhanaGeo. Keep `GHANAGEO_API_KEY` in server environment variables and never prefix it with `NEXT_PUBLIC_`.

```ts
// app/api/search/route.ts — server module
import { createServerClientFromEnv } from "@ghanageo/node";

const ghanageo = createServerClientFromEnv();
```

Client Components should import `@ghanageo/react`; only Browser-class keys with an origin allow-list may be exposed there.
