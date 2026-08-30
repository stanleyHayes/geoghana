using System.Net;
using System.Text.Json;

namespace GhanaGeo;

public sealed class GhanaGeoException : Exception
{
    public GhanaGeoException(string code, string message, HttpStatusCode? statusCode = null,
        string? requestId = null, IReadOnlyDictionary<string, JsonElement>? details = null, string? docs = null,
        Exception? innerException = null) : base(message, innerException)
    {
        Code = code;
        StatusCode = statusCode;
        RequestId = requestId;
        Details = details;
        Docs = docs;
    }

    public string Code { get; }
    public HttpStatusCode? StatusCode { get; }
    public string? RequestId { get; }
    public IReadOnlyDictionary<string, JsonElement>? Details { get; }
    public string? Docs { get; }
}
