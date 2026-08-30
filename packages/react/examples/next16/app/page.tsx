import { dehydrate, QueryClient } from "@tanstack/react-query";
import { GhanaGeoClient, ghanaGeoKeys } from "@ghanageo/react";
import { Providers } from "./providers";

export default async function Page() {
  const queryClient = new QueryClient();
  const client = new GhanaGeoClient();
  await queryClient.prefetchQuery({
    queryKey: ghanaGeoKeys.regions(),
    queryFn: ({ signal }) => client.regions({}, signal),
  });

  return <Providers state={dehydrate(queryClient)}>Your hydrated location UI</Providers>;
}
