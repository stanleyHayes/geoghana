# `@ghanageo/react`

React/TanStack Query access layer for GhanaGeo.

```tsx
import { GhanaGeoProvider, useSearch, useRegions } from "@ghanageo/react";

export function App() {
  return (
    <GhanaGeoProvider options={{ apiKey: process.env.NEXT_PUBLIC_GHANAGEO_KEY }}>
      <LocationSearch />
    </GhanaGeoProvider>
  );
}

function LocationSearch() {
  const regions = useRegions();
  const search = useSearch("Tema Comm 25");
  return <pre>{JSON.stringify(search.data ?? regions.data, null, 2)}</pre>;
}
```

First-class hooks:
`useRegions`, `useRegion`, `useDistricts`, `useDistrict`, `usePlace`, `useSearch`, `useAutocomplete`, `useGeocode`, `useReverseGeocode`, `useNearby`, and generic `useGhanaGeoGraphQL`.

The public package should use REST for the convenience hooks by default while preserving protocol-neutral server semantics. GraphQL gets a generic typed query hook. Browser gRPC should be exposed only through an officially supported Connect/gRPC-Web transport rather than pretending native HTTP/2 gRPC works in every browser.

For Next.js 16 server prefetching and client hydration, see
[`examples/next16`](./examples/next16). Keep server keys in server components;
only pass anonymous or explicitly public browser configuration to the client
provider.
