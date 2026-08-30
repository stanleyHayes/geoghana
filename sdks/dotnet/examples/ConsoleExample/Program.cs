using GhanaGeo;

using var http = new HttpClient();
var client = new GhanaGeoClient(http); // Public reads are anonymous by default.
await foreach (var page in client.EnumerateRegionPagesAsync(new ListOptions(Limit: 5)))
{
    foreach (var region in page.Data) Console.WriteLine($"{region.Name} ({region.Id})");
}
