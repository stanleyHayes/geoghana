using GhanaGeo;
using Microsoft.Extensions.DependencyInjection;

var services = new ServiceCollection();
services.AddGhanaGeo(options => options.TelemetryEnabled = false);
await using var provider = services.BuildServiceProvider();
var client = provider.GetRequiredService<IGhanaGeoClient>();
Console.WriteLine($"GhanaGeo targets {client.ApiVersion}; tested with {client.TestedDatasetVersion}.");
