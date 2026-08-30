using System.Diagnostics;
using System.Net;
using System.Net.Http.Headers;
using System.Runtime.CompilerServices;
using System.Security.Cryptography;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace GhanaGeo;

public sealed class GhanaGeoClient : IGhanaGeoClient
{
    public const string CurrentApiVersion = "v1";
    public const string CurrentTestedDatasetVersion = "2026.08.3-ulid";
    public const string SdkVersion = "1.0.0";

    private static readonly JsonSerializerOptions JsonOptions = new(JsonSerializerDefaults.Web)
    {
        PropertyNameCaseInsensitive = true,
        Converters = { new JsonStringEnumConverter(JsonNamingPolicy.SnakeCaseUpper) },
    };

    private readonly HttpClient httpClient;
    private readonly GhanaGeoOptions options;
    private readonly Func<TimeSpan, CancellationToken, Task> delay;

    public GhanaGeoClient(HttpClient httpClient, GhanaGeoOptions? options = null)
        : this(httpClient, options ?? new GhanaGeoOptions(), Task.Delay) { }

    internal GhanaGeoClient(HttpClient httpClient, GhanaGeoOptions options, Func<TimeSpan, CancellationToken, Task> delay)
    {
        ArgumentNullException.ThrowIfNull(httpClient);
        ArgumentNullException.ThrowIfNull(options);
        options.Validate();
        this.httpClient = httpClient;
        this.options = options;
        this.delay = delay;
        if (httpClient.BaseAddress is null) httpClient.BaseAddress = options.BaseAddress;
        if (httpClient.DefaultRequestHeaders.UserAgent.Count == 0)
            httpClient.DefaultRequestHeaders.UserAgent.ParseAdd($"GhanaGeo-dotnet/{SdkVersion}");
    }

    public string ApiVersion => CurrentApiVersion;
    public string TestedDatasetVersion => CurrentTestedDatasetVersion;

    public Task<Page<Region>> ListRegionsAsync(ListOptions? value = null, CancellationToken cancellationToken = default) => GetAsync<Page<Region>>("regions", Query(value), nameof(ListRegionsAsync), cancellationToken);
    public Task<Region> GetRegionAsync(string id, CancellationToken cancellationToken = default) => GetAsync<Region>($"regions/{Segment(id)}", null, nameof(GetRegionAsync), cancellationToken);
    public Task<Page<District>> ListRegionDistrictsAsync(string id, ListOptions? value = null, CancellationToken cancellationToken = default) => GetAsync<Page<District>>($"regions/{Segment(id)}/districts", Query(value), nameof(ListRegionDistrictsAsync), cancellationToken);
    public Task<Page<District>> ListDistrictsAsync(DistrictListOptions? value = null, CancellationToken cancellationToken = default) => GetAsync<Page<District>>("districts", Query(value), nameof(ListDistrictsAsync), cancellationToken);
    public Task<District> GetDistrictAsync(string id, CancellationToken cancellationToken = default) => GetAsync<District>($"districts/{Segment(id)}", null, nameof(GetDistrictAsync), cancellationToken);
    public Task<Page<Place>> ListDistrictPlacesAsync(string id, PlaceListOptions? value = null, CancellationToken cancellationToken = default) => GetAsync<Page<Place>>($"districts/{Segment(id)}/places", Query(value, false), nameof(ListDistrictPlacesAsync), cancellationToken);
    public Task<Page<Place>> ListPlacesAsync(PlaceListOptions? value = null, CancellationToken cancellationToken = default) => GetAsync<Page<Place>>("places", Query(value, true), nameof(ListPlacesAsync), cancellationToken);
    public Task<Place> GetPlaceAsync(string id, CancellationToken cancellationToken = default) => GetAsync<Place>($"places/{Segment(id)}", null, nameof(GetPlaceAsync), cancellationToken);
    public Task<Page<SearchResult>> SearchAsync(SearchOptions value, CancellationToken cancellationToken = default) => GetAsync<Page<SearchResult>>("search", Query(value), nameof(SearchAsync), cancellationToken);
    public Task<Page<SearchResult>> AutocompleteAsync(TextQueryOptions value, CancellationToken cancellationToken = default) => GetAsync<Page<SearchResult>>("autocomplete", Query(value), nameof(AutocompleteAsync), cancellationToken);
    public Task<Page<SearchResult>> GeocodeAsync(TextQueryOptions value, CancellationToken cancellationToken = default) => GetAsync<Page<SearchResult>>("geocode", Query(value), nameof(GeocodeAsync), cancellationToken);
    public Task<ReverseResult> ReverseGeocodeAsync(double latitude, double longitude, CancellationToken cancellationToken = default) => GetAsync<ReverseResult>("reverse", Q(("lat", latitude), ("lng", longitude)), nameof(ReverseGeocodeAsync), cancellationToken);
    public Task<Page<Place>> NearbyAsync(NearbyOptions value, CancellationToken cancellationToken = default) => GetAsync<Page<Place>>("nearby", Query(value), nameof(NearbyAsync), cancellationToken);
    public Task<BoundaryFeature> GetBoundaryAsync(string id, CancellationToken cancellationToken = default) => GetAsync<BoundaryFeature>($"boundaries/{Segment(id)}", null, nameof(GetBoundaryAsync), cancellationToken);
    public Task<DatasetPage> ListDatasetsAsync(CancellationToken cancellationToken = default) => GetAsync<DatasetPage>("datasets", null, nameof(ListDatasetsAsync), cancellationToken);
    public Task<DownloadList> ListDatasetDownloadsAsync(string version, CancellationToken cancellationToken = default) => GetAsync<DownloadList>($"datasets/{Segment(version)}/downloads", null, nameof(ListDatasetDownloadsAsync), cancellationToken);
    public Task<JsonElement> ListRoadsAsync(CancellationToken cancellationToken = default) => GetAsync<JsonElement>("roads", null, nameof(ListRoadsAsync), cancellationToken);
    public Task<JsonElement> ListPointsOfInterestAsync(CancellationToken cancellationToken = default) => GetAsync<JsonElement>("pois", null, nameof(ListPointsOfInterestAsync), cancellationToken);

    public async Task DownloadDatasetArtifactAsync(string version, string entity, string format, Stream destination, DatasetArtifactOptions? downloadOptions = null, CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(destination);
        if (!destination.CanWrite) throw new ArgumentException("Destination must be writable.", nameof(destination));
        downloadOptions ??= new DatasetArtifactOptions();
        if (downloadOptions.MaximumBytes <= 0) throw new ArgumentOutOfRangeException(nameof(downloadOptions));
        var path = $"datasets/{Segment(version)}/downloads/{Segment(entity)}.{Segment(format)}";
        using var response = await SendGetAsync(path, null, nameof(DownloadDatasetArtifactAsync), cancellationToken).ConfigureAwait(false);
        await using var source = await response.Content.ReadAsStreamAsync(cancellationToken).ConfigureAwait(false);
        using var hash = IncrementalHash.CreateHash(HashAlgorithmName.SHA256);
        var buffer = new byte[81_920];
        long total = 0;
        while (true)
        {
            var read = await source.ReadAsync(buffer, cancellationToken).ConfigureAwait(false);
            if (read == 0) break;
            total += read;
            if (total > downloadOptions.MaximumBytes)
                throw new GhanaGeoException("DOWNLOAD_TOO_LARGE", $"Dataset artifact exceeded {downloadOptions.MaximumBytes} bytes.");
            hash.AppendData(buffer, 0, read);
            await destination.WriteAsync(buffer.AsMemory(0, read), cancellationToken).ConfigureAwait(false);
        }
        var actual = Convert.ToHexString(hash.GetHashAndReset()).ToLowerInvariant();
        var expected = NormalizeChecksum(downloadOptions.ExpectedSha256 ?? GetChecksum(response));
        if (expected is not null && !CryptographicOperations.FixedTimeEquals(Convert.FromHexString(actual), Convert.FromHexString(expected)))
            throw new GhanaGeoException("CHECKSUM_MISMATCH", "Dataset artifact checksum did not match the expected SHA-256 digest.");
    }

    public async Task DownloadDatasetArtifactToFileAsync(string version, string entity, string format, string destinationPath, DatasetArtifactOptions? downloadOptions = null, CancellationToken cancellationToken = default)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(destinationPath);
        var fullPath = Path.GetFullPath(destinationPath);
        var directory = Path.GetDirectoryName(fullPath) ?? throw new ArgumentException("Destination path has no directory.", nameof(destinationPath));
        Directory.CreateDirectory(directory);
        var temporaryPath = Path.Combine(directory, $".{Path.GetFileName(fullPath)}.{Guid.NewGuid():N}.tmp");
        try
        {
            await using (var file = new FileStream(temporaryPath, FileMode.CreateNew, FileAccess.Write, FileShare.None, 81_920, FileOptions.Asynchronous | FileOptions.SequentialScan))
                await DownloadDatasetArtifactAsync(version, entity, format, file, downloadOptions, cancellationToken).ConfigureAwait(false);
            File.Move(temporaryPath, fullPath, true);
        }
        finally
        {
            if (File.Exists(temporaryPath)) File.Delete(temporaryPath);
        }
    }

    public IAsyncEnumerable<Page<Region>> EnumerateRegionPagesAsync(ListOptions? value = null, CancellationToken cancellationToken = default) =>
        EnumeratePagesAsync(cursor => ListRegionsAsync((value ?? new ListOptions()) with { Cursor = cursor }, cancellationToken), value?.Cursor, cancellationToken);

    public IAsyncEnumerable<Page<District>> EnumerateDistrictPagesAsync(DistrictListOptions? value = null, CancellationToken cancellationToken = default) =>
        EnumeratePagesAsync(cursor => ListDistrictsAsync((value ?? new DistrictListOptions()) with { Cursor = cursor }, cancellationToken), value?.Cursor, cancellationToken);

    public IAsyncEnumerable<Page<Place>> EnumeratePlacePagesAsync(PlaceListOptions? value = null, CancellationToken cancellationToken = default) =>
        EnumeratePagesAsync(cursor => ListPlacesAsync((value ?? new PlaceListOptions()) with { Cursor = cursor }, cancellationToken), value?.Cursor, cancellationToken);

    private static async IAsyncEnumerable<Page<T>> EnumeratePagesAsync<T>(Func<string?, Task<Page<T>>> fetch, string? cursor, [EnumeratorCancellation] CancellationToken cancellationToken)
    {
        var seen = new HashSet<string>(StringComparer.Ordinal);
        while (true)
        {
            cancellationToken.ThrowIfCancellationRequested();
            var page = await fetch(cursor).ConfigureAwait(false);
            yield return page;
            if (string.IsNullOrEmpty(page.NextCursor)) yield break;
            if (!seen.Add(page.NextCursor)) throw new GhanaGeoException("REPEATED_CURSOR", "The API returned a repeated pagination cursor.");
            cursor = page.NextCursor;
        }
    }

    private async Task<T> GetAsync<T>(string path, string? query, string operation, CancellationToken cancellationToken)
    {
        using var response = await SendGetAsync(path, query, operation, cancellationToken).ConfigureAwait(false);
        EnsureContentLength(response, options.MaximumJsonResponseBytes, "RESPONSE_TOO_LARGE");
        try
        {
            await using var content = await response.Content.ReadAsStreamAsync(cancellationToken).ConfigureAwait(false);
            await using var bounded = new BoundedReadStream(content, options.MaximumJsonResponseBytes);
            return (await JsonSerializer.DeserializeAsync<T>(bounded, JsonOptions, cancellationToken).ConfigureAwait(false))
                ?? throw new GhanaGeoException("INVALID_RESPONSE", "The API returned an empty JSON response.", response.StatusCode);
        }
        catch (ResponseLimitExceededException exception)
        {
            throw new GhanaGeoException("RESPONSE_TOO_LARGE", exception.Message, response.StatusCode, innerException: exception);
        }
        catch (JsonException exception)
        {
            throw new GhanaGeoException("INVALID_RESPONSE", "The API returned JSON that does not match the GhanaGeo contract.", response.StatusCode, innerException: exception);
        }
    }

    private async Task<HttpResponseMessage> SendGetAsync(string path, string? query, string operation, CancellationToken cancellationToken)
    {
        var uri = query is null ? path : $"{path}?{query}";
        for (var attempt = 0; ; attempt++)
        {
            cancellationToken.ThrowIfCancellationRequested();
            var stopwatch = Stopwatch.StartNew();
            try
            {
                using var request = new HttpRequestMessage(HttpMethod.Get, uri);
                request.Headers.Accept.Add(new MediaTypeWithQualityHeaderValue("application/json"));
                if (!string.IsNullOrWhiteSpace(options.ApiKey)) request.Headers.Authorization = new AuthenticationHeaderValue("Bearer", options.ApiKey);
                var response = await httpClient.SendAsync(request, HttpCompletionOption.ResponseHeadersRead, cancellationToken).ConfigureAwait(false);
                stopwatch.Stop();
                Emit(operation, attempt, (int)response.StatusCode, stopwatch.Elapsed, response.IsSuccessStatusCode ? "success" : "http_error");
                if (response.IsSuccessStatusCode) return response;
                if (attempt < options.MaximumRetries && IsRetryable(response.StatusCode))
                {
                    var retryDelay = RetryDelay(response, attempt);
                    response.Dispose();
                    await delay(retryDelay, cancellationToken).ConfigureAwait(false);
                    continue;
                }
                await ThrowApiErrorAsync(response, cancellationToken).ConfigureAwait(false);
            }
            catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested) { throw; }
            catch (HttpRequestException) when (attempt < options.MaximumRetries)
            {
                stopwatch.Stop();
                Emit(operation, attempt, null, stopwatch.Elapsed, "transport_error");
                await delay(Backoff(attempt), cancellationToken).ConfigureAwait(false);
                continue;
            }
            catch (HttpRequestException exception)
            {
                stopwatch.Stop();
                Emit(operation, attempt, null, stopwatch.Elapsed, "transport_error");
                throw new GhanaGeoException("TRANSPORT_ERROR", "The GhanaGeo request could not be completed.", innerException: exception);
            }
        }
    }

    private async Task ThrowApiErrorAsync(HttpResponseMessage response, CancellationToken cancellationToken)
    {
        try
        {
            EnsureContentLength(response, options.MaximumErrorResponseBytes, "ERROR_RESPONSE_TOO_LARGE");
            await using var content = await response.Content.ReadAsStreamAsync(cancellationToken).ConfigureAwait(false);
            await using var body = new BoundedReadStream(content, options.MaximumErrorResponseBytes);
            var envelope = await JsonSerializer.DeserializeAsync<ErrorEnvelope>(body, JsonOptions, cancellationToken).ConfigureAwait(false);
            if (envelope?.Error is not null)
                throw new GhanaGeoException(envelope.Error.Code, envelope.Error.Message, response.StatusCode, envelope.Error.RequestId, envelope.Error.Details, envelope.Error.Docs);
        }
        catch (ResponseLimitExceededException exception)
        {
            throw new GhanaGeoException("ERROR_RESPONSE_TOO_LARGE", exception.Message, response.StatusCode, innerException: exception);
        }
        catch (JsonException) { }
        finally { response.Dispose(); }
        throw new GhanaGeoException("HTTP_ERROR", $"GhanaGeo returned HTTP {(int)response.StatusCode}.", response.StatusCode);
    }

    private void Emit(string operation, int attempt, int? status, TimeSpan duration, string outcome)
    {
        if (!options.TelemetryEnabled || options.TelemetrySink is null) return;
        try { options.TelemetrySink(new GhanaGeoTelemetryEvent(operation, attempt + 1, status, duration, outcome)); }
        catch { /* Observers cannot alter transport behavior. */ }
    }

    private static bool IsRetryable(HttpStatusCode status) => status is HttpStatusCode.TooManyRequests or HttpStatusCode.BadGateway or HttpStatusCode.ServiceUnavailable or HttpStatusCode.GatewayTimeout;
    private static void EnsureContentLength(HttpResponseMessage response, long maximumBytes, string code)
    {
        if (response.Content.Headers.ContentLength is { } length && length > maximumBytes)
            throw new GhanaGeoException(code, $"Response Content-Length {length} exceeds the configured {maximumBytes}-byte limit.", response.StatusCode);
    }
    private TimeSpan RetryDelay(HttpResponseMessage response, int attempt)
    {
        var retry = response.Headers.RetryAfter;
        var value = retry?.Delta ?? (retry?.Date is { } date ? date - DateTimeOffset.UtcNow : null);
        return value is { } delayValue && delayValue > TimeSpan.Zero
            ? TimeSpan.FromMilliseconds(Math.Min(delayValue.TotalMilliseconds, options.MaximumRetryDelay.TotalMilliseconds))
            : Backoff(attempt);
    }

    private TimeSpan Backoff(int attempt)
    {
        var ceiling = Math.Min(options.MaximumRetryDelay.TotalMilliseconds, 250 * Math.Pow(2, attempt));
        return TimeSpan.FromMilliseconds(Random.Shared.NextDouble() * ceiling);
    }

    private static string Segment(string value)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(value);
        return Uri.EscapeDataString(value);
    }

    private static string? GetChecksum(HttpResponseMessage response) =>
        response.Headers.TryGetValues("x-checksum-sha256", out var values) ? values.FirstOrDefault() : null;

    private static string? NormalizeChecksum(string? value)
    {
        if (string.IsNullOrWhiteSpace(value)) return null;
        var normalized = value.Trim().Replace("sha256:", "", StringComparison.OrdinalIgnoreCase);
        if (normalized.Length != 64 || normalized.Any(c => !Uri.IsHexDigit(c))) throw new ArgumentException("ExpectedSha256 must be a 64-character hexadecimal SHA-256 digest.");
        return normalized.ToLowerInvariant();
    }

    private static string Query(ListOptions? value) => value is null ? "" : Q(("cursor", value.Cursor), ("limit", value.Limit));
    private static string Query(DistrictListOptions? value) => value is null ? "" : Q(("cursor", value.Cursor), ("limit", value.Limit), ("regionId", value.RegionId), ("q", value.Query));
    private static string Query(PlaceListOptions? value, bool includeDistrict) => value is null ? "" : Q(("cursor", value.Cursor), ("limit", value.Limit), ("districtId", includeDistrict ? value.DistrictId : null), ("regionId", value.RegionId), ("type", value.Type is { } type ? JsonNamingPolicy.SnakeCaseUpper.ConvertName(type.ToString()) : null), ("q", value.Query));
    private static string Query(SearchOptions value) { ArgumentNullException.ThrowIfNull(value); return Q(("q", value.Query), ("limit", value.Limit), ("regionId", value.RegionId), ("districtId", value.DistrictId), ("type", value.Type is { } type ? JsonNamingPolicy.SnakeCaseUpper.ConvertName(type.ToString()) : null)); }
    private static string Query(TextQueryOptions value) { ArgumentNullException.ThrowIfNull(value); return Q(("q", value.Query), ("limit", value.Limit)); }
    private static string Query(NearbyOptions value) { ArgumentNullException.ThrowIfNull(value); return Q(("lat", value.Latitude), ("lng", value.Longitude), ("radius", value.Radius), ("limit", value.Limit)); }
    private static string Q(params (string Key, object? Value)[] values) => string.Join("&", values.Where(value => value.Value is not null).Select(value => $"{Uri.EscapeDataString(value.Key)}={Uri.EscapeDataString(Convert.ToString(value.Value, System.Globalization.CultureInfo.InvariantCulture)!)}"));

    private sealed record ErrorEnvelope(ApiError Error);
    private sealed record ApiError(string Code, string Message, string RequestId, string Docs, IReadOnlyDictionary<string, JsonElement>? Details = null);
}
