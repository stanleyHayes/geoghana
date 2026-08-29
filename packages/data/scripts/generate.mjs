import { readFile, writeFile } from "node:fs/promises";
import { parse } from "csv-parse/sync";

const version = "2026.08.3-ulid";
const root = new URL(`../../../data/exports/${version}/`, import.meta.url);
const read = async (name) => parse(await readFile(new URL(name, root), "utf8"), { columns: true, skip_empty_lines: true });
const [regions, districts, places] = await Promise.all([read("regions.csv"), read("districts.csv"), read("places.csv")]);
for (const place of places) {
  place.latitude = place.latitude ? Number(place.latitude) : null;
  place.longitude = place.longitude ? Number(place.longitude) : null;
  place.population = place.population ? Number(place.population) : null;
}
await writeFile(new URL("../src/data.json", import.meta.url), `${JSON.stringify({ datasetVersion: version, regions, districts, places })}\n`);
console.log(`generated offline ${version}: ${regions.length} regions, ${districts.length} districts, ${places.length} places`);
