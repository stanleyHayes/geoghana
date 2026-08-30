using System.Runtime.CompilerServices;
using System.Text.Json;

namespace GhanaGeo;

public interface IGhanaGeoClient
{
    string ApiVersion { get; }
    string TestedDatasetVersion { get; }
    Task<Page<Region>> ListRegionsAsync(ListOptions? value = null, CancellationToken cancellationToken = default);
    Task<Region> GetRegionAsync(string id, CancellationToken cancellationToken = default);
    Task<Page<District>> ListRegionDistrictsAsync(string id, ListOptions? value = null, CancellationToken cancellationToken = default);
    Task<Page<District>> ListDistrictsAsync(DistrictListOptions? value = null, CancellationToken cancellationToken = default);
    Task<District> GetDistrictAsync(string id, CancellationToken cancellationToken = default);
    Task<Page<Place>> ListDistrictPlacesAsync(string id, PlaceListOptions? value = null, CancellationToken cancellationToken = default);
    Task<Page<Place>> ListPlacesAsync(PlaceListOptions? value = null, CancellationToken cancellationToken = default);
    Task<Place> GetPlaceAsync(string id, CancellationToken cancellationToken = default);
    Task<Page<SearchResult>> SearchAsync(SearchOptions value, CancellationToken cancellationToken = default);
    Task<Page<SearchResult>> AutocompleteAsync(TextQueryOptions value, CancellationToken cancellationToken = default);
    Task<Page<SearchResult>> GeocodeAsync(TextQueryOptions value, CancellationToken cancellationToken = default);
    Task<ReverseResult> ReverseGeocodeAsync(double latitude, double longitude, CancellationToken cancellationToken = default);
    Task<Page<Place>> NearbyAsync(NearbyOptions value, CancellationToken cancellationToken = default);
    Task<BoundaryFeature> GetBoundaryAsync(string id, CancellationToken cancellationToken = default);
    Task<DatasetPage> ListDatasetsAsync(CancellationToken cancellationToken = default);
    Task<DownloadList> ListDatasetDownloadsAsync(string version, CancellationToken cancellationToken = default);
    Task<JsonElement> ListRoadsAsync(CancellationToken cancellationToken = default);
    Task<JsonElement> ListPointsOfInterestAsync(CancellationToken cancellationToken = default);
    Task DownloadDatasetArtifactAsync(string version, string entity, string format, Stream destination, DatasetArtifactOptions? downloadOptions = null, CancellationToken cancellationToken = default);
    Task DownloadDatasetArtifactToFileAsync(string version, string entity, string format, string destinationPath, DatasetArtifactOptions? downloadOptions = null, CancellationToken cancellationToken = default);
    IAsyncEnumerable<Page<Region>> EnumerateRegionPagesAsync(ListOptions? value = null, CancellationToken cancellationToken = default);
    IAsyncEnumerable<Page<District>> EnumerateDistrictPagesAsync(DistrictListOptions? value = null, CancellationToken cancellationToken = default);
    IAsyncEnumerable<Page<Place>> EnumeratePlacePagesAsync(PlaceListOptions? value = null, CancellationToken cancellationToken = default);
}
