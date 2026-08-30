import { GhanaGeoClient, GhanaGeoError } from "@ghanageo/client";

export async function quickStart(signal?: AbortSignal) {
  const client = new GhanaGeoClient();

  try {
    const page = await client.search("Kumasi", { limit: 5 }, signal);
    for (const place of page.data) console.log(place.name);
  } catch (error) {
    if (error instanceof GhanaGeoError) console.error(error.code, error.requestId);
    else throw error;
  }
}
