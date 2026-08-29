# @ghanageo/data

Offline, network-free access to GhanaGeo's published region, district and place corpus.

```ts
import { datasetVersion, getRegions, search } from "@ghanageo/data";
```

The code version follows SemVer; `datasetVersion` reports the independent data release. Geometry, roads and POIs are intentionally excluded from the base package to keep installs practical.
