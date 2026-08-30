using Microsoft.Extensions.DependencyInjection;

namespace GhanaGeo;

public static class GhanaGeoServiceCollectionExtensions
{
    public static IHttpClientBuilder AddGhanaGeo(this IServiceCollection services, Action<GhanaGeoOptions>? configure = null)
    {
        ArgumentNullException.ThrowIfNull(services);
        var options = new GhanaGeoOptions();
        configure?.Invoke(options);
        options.Validate();
        services.AddSingleton(options);
        return services.AddHttpClient<IGhanaGeoClient, GhanaGeoClient>(client => client.BaseAddress = options.BaseAddress);
    }
}
