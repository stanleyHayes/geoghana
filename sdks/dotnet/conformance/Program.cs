using System.Diagnostics;
using System.Net;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using GhanaGeo;

var settings = Settings.Parse(args);
var exportedJson = ExportContract(settings.RepositoryRoot);
using var exported = JsonDocument.Parse(exportedJson);
var contractDigest = exported.RootElement.GetProperty("contractDigest").GetString() ?? throw new InvalidOperationException("Contract digest absent.");
var results = new List<Result>();
var evidence = new List<Evidence>();
var versions = new HashSet<string>(StringComparer.Ordinal);

foreach (var contractCase in exported.RootElement.GetProperty("cases").EnumerateArray().Where(HasRest))
{
    var id = contractCase.GetProperty("id").GetString()!;
    var handler = new CaptureHandler(id);
    var auth = GetString(contractCase, "input", "auth");
    var options = new GhanaGeoOptions { BaseAddress = settings.BaseUrl, ApiKey = auth is null or "omitted" ? null : auth, MaximumRetries = 0, TelemetryEnabled = false };
    var client = new GhanaGeoClient(new HttpClient(handler) { BaseAddress = settings.BaseUrl }, options);
    object? value = null;
    Exception? failure = null;
    try { value = await Invoke(client, contractCase); }
    catch (OperationCanceledException) when (id == "semantic.cancellation-propagates") { value = new { cancelled = true }; }
    catch (Exception exception) { failure = exception; }

    var normalized = failure is GhanaGeoException apiError
        ? JsonSerializer.SerializeToElement(new { error = new { code = apiError.Code, message = apiError.Message, requestId = apiError.RequestId, docs = apiError.Docs, details = apiError.Details } }, JsonDefaults.Options)
        : failure is null ? JsonSerializer.SerializeToElement(value, JsonDefaults.Options) : JsonSerializer.SerializeToElement(new { runnerFailure = failure.ToString() }, JsonDefaults.Options);
    FindDatasetVersions(normalized, versions);
    var failures = Validate(contractCase, normalized, failure, handler);
    results.Add(new(id, failures.Count == 0 ? "passed" : "failed", ["rest"], failures.Count == 0 ? null : string.Join("; ", failures)));
    evidence.Add(new(id, "rest", Digest(handler.RequestEvidence), Digest(handler.ResponseEvidence)));
}

var orderedEvidence = evidence.OrderBy(item => item.CaseId, StringComparer.Ordinal).ToArray();
var report = new
{
    schemaVersion = 2,
    contract = new { digest = contractDigest, runnerVersion = "1.0.0" },
    sdk = new { language = "C#", name = "GhanaGeo", version = GhanaGeoClient.SdkVersion, supportedProtocols = new[] { "rest" } },
    apiVersion = GhanaGeoClient.CurrentApiVersion,
    datasetVersion = versions.Order(StringComparer.Ordinal).FirstOrDefault() ?? GhanaGeoClient.CurrentTestedDatasetVersion,
    summary = new { passed = results.Count(item => item.Status == "passed"), failed = results.Count(item => item.Status == "failed"), skipped = 0 },
    evidenceDigest = Digest(orderedEvidence),
    evidence = orderedEvidence,
    results,
};
var reportJson = JsonSerializer.Serialize(report, JsonDefaults.Indented);
if (settings.ReportPath is not null) await File.WriteAllTextAsync(settings.ReportPath, reportJson);
Console.WriteLine(reportJson);
return results.Any(item => item.Status == "failed") ? 1 : 0;

static async Task<object?> Invoke(GhanaGeoClient client, JsonElement item)
{
    var operation = item.GetProperty("operation").GetString();
    var input = item.TryGetProperty("input", out var inputValue) ? inputValue : default;
    var path = input.ValueKind == JsonValueKind.Object && input.TryGetProperty("path", out var pathValue) ? pathValue : default;
    var query = input.ValueKind == JsonValueKind.Object && input.TryGetProperty("query", out var queryValue) ? queryValue : default;
    string? S(JsonElement element, string name) => element.ValueKind == JsonValueKind.Object && element.TryGetProperty(name, out var value) ? value.ToString() : null;
    int? I(JsonElement element, string name) => int.TryParse(S(element, name), out var value) ? value : null;
    double D(JsonElement element, string name) => double.TryParse(S(element, name), System.Globalization.NumberStyles.Float, System.Globalization.CultureInfo.InvariantCulture, out var value) ? value : 0;
    var page = new ListOptions(S(query, "cursor"), I(query, "limit"));
    if (item.GetProperty("id").GetString() == "semantic.cancellation-propagates")
    {
        using var cancel = new CancellationTokenSource();
        cancel.CancelAfter(I(input, "cancelAfterMilliseconds") ?? 10);
        return await client.SearchAsync(new SearchOptions("Kumasi"), cancel.Token);
    }
    return operation switch
    {
        "listRegions" => await client.ListRegionsAsync(page),
        "getRegion" => await client.GetRegionAsync(S(path, "id")!),
        "listRegionDistricts" => await client.ListRegionDistrictsAsync(S(path, "id")!, page),
        "listDistricts" => await client.ListDistrictsAsync(new(S(query, "cursor"), I(query, "limit"), S(query, "regionId"), S(query, "q"))),
        "getDistrict" => await client.GetDistrictAsync(S(path, "id")!),
        "listDistrictPlaces" => await client.ListDistrictPlacesAsync(S(path, "id")!, new(S(query, "cursor"), I(query, "limit"), Type: ParsePlaceType(S(query, "type")))),
        "listPlaces" => await client.ListPlacesAsync(new(S(query, "cursor"), I(query, "limit"), S(query, "districtId"), S(query, "regionId"), ParsePlaceType(S(query, "type")), S(query, "q"))),
        "getPlace" => await client.GetPlaceAsync(S(path, "id")!),
        "search" => await client.SearchAsync(new(S(query, "q") ?? "", I(query, "limit"), S(query, "regionId"))),
        "autocomplete" => await client.AutocompleteAsync(new(S(query, "q") ?? "", I(query, "limit"))),
        "geocode" => await client.GeocodeAsync(new(S(query, "q") ?? "", I(query, "limit"))),
        "reverseGeocode" => await client.ReverseGeocodeAsync(D(query, "lat"), D(query, "lng")),
        "nearby" => await client.NearbyAsync(new(D(query, "lat"), D(query, "lng"), D(query, "radius"), I(query, "limit"))),
        "getBoundary" => await client.GetBoundaryAsync(S(path, "id")!),
        "listDatasets" => await client.ListDatasetsAsync(),
        "listDatasetDownloads" => await client.ListDatasetDownloadsAsync(S(path, "version")!),
        "downloadDatasetArtifact" => await Download(client, S(path, "version")!, S(path, "entity")!, S(path, "format")!),
        "listRoads" => await client.ListRoadsAsync(),
        "listPointsOfInterest" => await client.ListPointsOfInterestAsync(),
        _ => throw new InvalidOperationException($"No .NET REST mapping for {operation}.")
    };
}

static async Task<object> Download(GhanaGeoClient client, string version, string entity, string format)
{
    await using var stream = new MemoryStream();
    await client.DownloadDatasetArtifactAsync(version, entity, format, stream);
    return new { bytesSha256 = Convert.ToHexString(SHA256.HashData(stream.ToArray())).ToLowerInvariant() };
}

static List<string> Validate(JsonElement item, JsonElement value, Exception? failure, CaptureHandler handler)
{
    var failures = new List<string>();
    var expected = item.GetProperty("expect");
    var outcome = expected.GetProperty("outcome").GetString();
    var actualOutcome = failure is null ? "success" : "error";
    if (outcome != actualOutcome) failures.Add($"expected {outcome}, received {actualOutcome}");
    if (expected.TryGetProperty("httpStatus", out var status) && status.GetInt32() != handler.Status) failures.Add($"expected HTTP {status.GetInt32()}, received {handler.Status}");
    if (expected.TryGetProperty("shape", out var shape) && shape.TryGetProperty("required", out var required))
        foreach (var field in required.EnumerateArray().Select(field => field.GetString()!))
            if (!TryPath(value, field, out _)) failures.Add($"missing field {field}");
    if (expected.TryGetProperty("error", out var expectedError) && expectedError.TryGetProperty("code", out var code))
    {
        if (failure is not GhanaGeoException error || error.Code != code.GetString()) failures.Add("typed catalog error mismatch");
        if (expectedError.TryGetProperty("requiredFields", out var fields))
            foreach (var field in fields.EnumerateArray().Select(field => field.GetString()!))
                if (!TryPath(value, $"error.{field}", out _)) failures.Add($"missing error field {field}");
    }
    if (item.GetProperty("id").GetString() == "semantic.anonymous-default" && handler.AuthorizationSent) failures.Add("anonymous request sent authorization");
    if (expected.TryGetProperty("semantics", out var semantics) && semantics.EnumerateArray().Any(value => value.GetString() == "datasetVersionPresent") && (!TryPath(value, "datasetVersion", out var version) || string.IsNullOrWhiteSpace(version.GetString()))) failures.Add("datasetVersion absent");
    return failures;
}

static bool TryPath(JsonElement value, string path, out JsonElement found)
{
    found = value;
    foreach (var segment in path.Split('.')) if (found.ValueKind != JsonValueKind.Object || !found.TryGetProperty(segment, out found)) return false;
    return true;
}
static bool HasRest(JsonElement item) => item.GetProperty("protocols").EnumerateArray().Any(value => value.GetString() == "rest");
static string? GetString(JsonElement item, params string[] path) => path.Aggregate((JsonElement?)item, (current, part) => current is { ValueKind: JsonValueKind.Object } value && value.TryGetProperty(part, out var next) ? next : null)?.ToString();
static PlaceType? ParsePlaceType(string? value) => Enum.TryParse<PlaceType>(value?.Replace("_", ""), true, out var type) ? type : null;
static void FindDatasetVersions(JsonElement value, HashSet<string> versions)
{
    if (value.ValueKind == JsonValueKind.Object) foreach (var property in value.EnumerateObject()) { if (property.NameEquals("datasetVersion") && property.Value.ValueKind == JsonValueKind.String) versions.Add(property.Value.GetString()!); FindDatasetVersions(property.Value, versions); }
    else if (value.ValueKind == JsonValueKind.Array) foreach (var item in value.EnumerateArray()) FindDatasetVersions(item, versions);
}
static string Digest(object? value) => Convert.ToHexString(SHA256.HashData(Encoding.UTF8.GetBytes(JsonSerializer.Serialize(value, JsonDefaults.Options)))).ToLowerInvariant();
static string ExportContract(string root)
{
    using var process = Process.Start(new ProcessStartInfo("ruby", "tools/conformance/export_cases.rb") { WorkingDirectory = root, RedirectStandardOutput = true, RedirectStandardError = true }) ?? throw new InvalidOperationException("Could not start contract exporter.");
    var output = process.StandardOutput.ReadToEnd(); var error = process.StandardError.ReadToEnd(); process.WaitForExit();
    return process.ExitCode == 0 ? output : throw new InvalidOperationException($"Contract export failed: {error}");
}

sealed class CaptureHandler(string caseId) : DelegatingHandler(new HttpClientHandler())
{
    public int Status { get; private set; }
    public bool AuthorizationSent { get; private set; }
    public object RequestEvidence { get; private set; } = new { };
    public object ResponseEvidence { get; private set; } = new { };
    protected override async Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
    {
        if (caseId == "error.invalid-argument" && request.RequestUri is { } original)
        {
            var builder = new UriBuilder(original) { Query = "limit=invalid" };
            request.RequestUri = builder.Uri;
        }
        request.Headers.TryAddWithoutValidation("x-conformance-case", caseId);
        AuthorizationSent = request.Headers.Authorization is not null;
        RequestEvidence = new { method = request.Method.Method, path = request.RequestUri?.AbsolutePath, query = request.RequestUri?.Query, authorizationSent = AuthorizationSent };
        var response = await base.SendAsync(request, cancellationToken);
        Status = (int)response.StatusCode;
        var body = await response.Content.ReadAsByteArrayAsync(cancellationToken);
        ResponseEvidence = new { status = Status, bodySha256 = Convert.ToHexString(SHA256.HashData(body)).ToLowerInvariant(), quota = response.Headers.TryGetValues("x-conformance-quota", out var quota) ? quota.FirstOrDefault() : null };
        var replacement = new ByteArrayContent(body);
        foreach (var header in response.Content.Headers) replacement.Headers.TryAddWithoutValidation(header.Key, header.Value);
        response.Content = replacement;
        return response;
    }
}

sealed record Result(string CaseId, string Status, string[] Protocols, string? Message = null);
sealed record Evidence(string CaseId, string Protocol, string RequestDigest, string ResponseDigest);
sealed record Settings(Uri BaseUrl, string RepositoryRoot, string? ReportPath)
{
    public static Settings Parse(string[] args)
    {
        string? Value(string name) { var index = Array.IndexOf(args, name); return index >= 0 && index + 1 < args.Length ? args[index + 1] : null; }
        var root = Path.GetFullPath(Value("--root") ?? Path.Combine(AppContext.BaseDirectory, "../../../../../../"));
        return new(new Uri(Value("--base-url") ?? "http://localhost:8180/v1/"), root, Value("--report"));
    }
}
static class JsonDefaults
{
    public static readonly JsonSerializerOptions Options = new(JsonSerializerDefaults.Web) { Converters = { new System.Text.Json.Serialization.JsonStringEnumConverter(JsonNamingPolicy.SnakeCaseUpper) } };
    public static readonly JsonSerializerOptions Indented = new(Options) { WriteIndented = true, DefaultIgnoreCondition = System.Text.Json.Serialization.JsonIgnoreCondition.WhenWritingNull };
}
