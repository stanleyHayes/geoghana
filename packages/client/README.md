# @ghanageo/client

Framework-neutral REST and GraphQL access for GhanaGeo. The client works anonymously by default and forwards `AbortSignal` to every request.

```ts
import { GhanaGeoClient } from "@ghanageo/client";

const client = new GhanaGeoClient();
const results = await client.search("Tema Community 25");
```
