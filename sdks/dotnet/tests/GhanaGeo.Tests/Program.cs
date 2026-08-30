using System.Net;
using System.Security.Cryptography;
using System.Text;
using GhanaGeo;

var tests = new (string Name, Func<Task> Run)[]
{
    ("anonymous typed request", AnonymousTypedRequest),
    ("typed catalog error", TypedError),
    ("bounded retry honors Retry-After", Retry),
    ("cancellation propagates", Cancellation),
    ("lazy pagination rejects repeated cursor", Pagination),
    ("bounded checksummed download", Download),
    ("success Content-Length is bounded", OversizedSuccess),
    ("streamed error body is bounded", OversizedError),
};
foreach (var test in tests)
{
    await test.Run();
    Console.WriteLine($"PASS {test.Name}");
}

static async Task AnonymousTypedRequest()
{
    var handler = new StubHandler((request, _) =>
    {
        Assert(request.RequestUri?.PathAndQuery == "/v1/regions?limit=2", "query mapping");
        Assert(request.Headers.Authorization is null, "anonymous by default");
        return Json(HttpStatusCode.OK, """{"data":[],"datasetVersion":"fixture"}""");
    });
    var client = Client(handler);
    var page = await client.ListRegionsAsync(new ListOptions(Limit: 2));
    Assert(page.DatasetVersion == "fixture", "typed response");
}

static async Task TypedError()
{
    var handler = new StubHandler((_, _) => Json(HttpStatusCode.NotFound, """{"error":{"code":"NOT_FOUND","message":"missing","requestId":"req-1","docs":"/errors/not-found","details":{"id":"x"}}}"""));
    try { await Client(handler).GetPlaceAsync("x"); throw new InvalidOperationException("expected error"); }
    catch (GhanaGeoException error)
    {
        Assert(error.Code == "NOT_FOUND" && error.RequestId == "req-1" && error.StatusCode == HttpStatusCode.NotFound, "typed error fields");
    }
}

static async Task Retry()
{
    var waits = new List<TimeSpan>();
    var handler = new StubHandler((_, call) => call < 3
        ? Response(HttpStatusCode.ServiceUnavailable, headers: new() { ["Retry-After"] = "1" })
        : Json(HttpStatusCode.OK, """{"data":[],"datasetVersion":"fixture"}"""));
    var options = new GhanaGeoOptions { BaseAddress = new("https://fixture.test/v1/"), MaximumRetries = 2 };
    var client = new GhanaGeoClient(new HttpClient(handler) { BaseAddress = options.BaseAddress }, options, (delay, _) => { waits.Add(delay); return Task.CompletedTask; });
    await client.ListRegionsAsync();
    Assert(handler.Calls == 3 && waits.Count == 2 && waits.All(value => value == TimeSpan.FromSeconds(1)), "retry contract");
}

static async Task Cancellation()
{
    var handler = new StubHandler(async (_, token) => { await Task.Delay(TimeSpan.FromMinutes(1), token); return Response(HttpStatusCode.OK); });
    using var cancellation = new CancellationTokenSource(TimeSpan.FromMilliseconds(10));
    try { await Client(handler).SearchAsync(new SearchOptions("Kumasi"), cancellation.Token); throw new InvalidOperationException("expected cancellation"); }
    catch (OperationCanceledException) when (cancellation.IsCancellationRequested) { }
}

static async Task Pagination()
{
    var handler = new StubHandler((_, _) => Json(HttpStatusCode.OK, """{"data":[],"datasetVersion":"fixture","nextCursor":"opaque-cursor"}"""));
    try
    {
        await foreach (var _ in Client(handler).EnumerateRegionPagesAsync()) { }
        throw new InvalidOperationException("expected repeated cursor error");
    }
    catch (GhanaGeoException error) { Assert(error.Code == "REPEATED_CURSOR" && handler.Calls == 2, "cursor guard"); }
}

static async Task Download()
{
    var bytes = Encoding.UTF8.GetBytes("ghana");
    var checksum = Convert.ToHexString(SHA256.HashData(bytes)).ToLowerInvariant();
    var handler = new StubHandler((_, _) => Response(HttpStatusCode.OK, bytes, new() { ["x-checksum-sha256"] = checksum }));
    await using var destination = new MemoryStream();
    await Client(handler).DownloadDatasetArtifactAsync("v", "regions", "json", destination, new DatasetArtifactOptions(32));
    Assert(destination.ToArray().SequenceEqual(bytes), "download bytes");
}

static async Task OversizedSuccess()
{
    var handler = new StubHandler((_, _) => Json(HttpStatusCode.OK, """{"data":[],"datasetVersion":"fixture"}"""));
    var options = new GhanaGeoOptions { BaseAddress = new("https://fixture.test/v1/"), MaximumJsonResponseBytes = 8 };
    try
    {
        await new GhanaGeoClient(new HttpClient(handler) { BaseAddress = options.BaseAddress }, options).ListRegionsAsync();
        throw new InvalidOperationException("expected response size error");
    }
    catch (GhanaGeoException error) { Assert(error.Code == "RESPONSE_TOO_LARGE", "success body cap"); }
}

static async Task OversizedError()
{
    var bytes = Encoding.UTF8.GetBytes("""{"error":{"code":"INTERNAL","message":"too large","requestId":"req","docs":"/errors/internal","details":{"padding":"xxxxxxxxxxxxxxxx"}}}""");
    var handler = new StubHandler((_, _) => new HttpResponseMessage(HttpStatusCode.InternalServerError)
    {
        Content = new StreamContent(new NonSeekableReadStream(bytes)),
    });
    var options = new GhanaGeoOptions { BaseAddress = new("https://fixture.test/v1/"), MaximumErrorResponseBytes = 32 };
    try
    {
        await new GhanaGeoClient(new HttpClient(handler) { BaseAddress = options.BaseAddress }, options).ListRegionsAsync();
        throw new InvalidOperationException("expected error response size error");
    }
    catch (GhanaGeoException error) { Assert(error.Code == "ERROR_RESPONSE_TOO_LARGE", "streaming error body cap"); }
}

static GhanaGeoClient Client(HttpMessageHandler handler) => new(new HttpClient(handler) { BaseAddress = new("https://fixture.test/v1/") }, new GhanaGeoOptions { BaseAddress = new("https://fixture.test/v1/") });
static HttpResponseMessage Json(HttpStatusCode status, string json) => Response(status, Encoding.UTF8.GetBytes(json), contentType: "application/json");
static HttpResponseMessage Response(HttpStatusCode status, byte[]? content = null, Dictionary<string, string>? headers = null, string? contentType = null)
{
    var response = new HttpResponseMessage(status) { Content = new ByteArrayContent(content ?? []) };
    if (contentType is not null) response.Content.Headers.ContentType = new(contentType);
    if (headers is not null) foreach (var header in headers) response.Headers.TryAddWithoutValidation(header.Key, header.Value);
    return response;
}
static void Assert(bool condition, string message) { if (!condition) throw new InvalidOperationException($"Assertion failed: {message}"); }

sealed class StubHandler : HttpMessageHandler
{
    private readonly Func<HttpRequestMessage, int, CancellationToken, Task<HttpResponseMessage>> response;
    public int Calls { get; private set; }
    public StubHandler(Func<HttpRequestMessage, int, HttpResponseMessage> response) => this.response = (request, call, _) => Task.FromResult(response(request, call));
    public StubHandler(Func<HttpRequestMessage, CancellationToken, Task<HttpResponseMessage>> response) => this.response = (request, _, token) => response(request, token);
    protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken) => response(request, ++Calls, cancellationToken);
}

sealed class NonSeekableReadStream(byte[] bytes) : MemoryStream(bytes)
{
    public override bool CanSeek => false;
}
