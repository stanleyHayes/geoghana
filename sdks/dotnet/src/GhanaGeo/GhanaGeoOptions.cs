namespace GhanaGeo;

public sealed class GhanaGeoOptions
{
    public Uri BaseAddress { get; set; } = new("https://api-geo.digitalghana.dev/v1/");
    public string? ApiKey { get; set; }
    public int MaximumRetries { get; set; } = 2;
    public TimeSpan MaximumRetryDelay { get; set; } = TimeSpan.FromSeconds(30);
    public long MaximumJsonResponseBytes { get; set; } = 8 * 1024 * 1024;
    public long MaximumErrorResponseBytes { get; set; } = 1024 * 1024;
    public bool TelemetryEnabled { get; set; } = true;
    public Action<GhanaGeoTelemetryEvent>? TelemetrySink { get; set; }

    internal void Validate()
    {
        if (!BaseAddress.IsAbsoluteUri) throw new ArgumentException("BaseAddress must be absolute.", nameof(BaseAddress));
        if (MaximumRetries is < 0 or > 5) throw new ArgumentOutOfRangeException(nameof(MaximumRetries), "Retries must be between zero and five.");
        if (MaximumRetryDelay <= TimeSpan.Zero) throw new ArgumentOutOfRangeException(nameof(MaximumRetryDelay));
        if (MaximumJsonResponseBytes <= 0) throw new ArgumentOutOfRangeException(nameof(MaximumJsonResponseBytes));
        if (MaximumErrorResponseBytes <= 0) throw new ArgumentOutOfRangeException(nameof(MaximumErrorResponseBytes));
    }
}

public sealed record GhanaGeoTelemetryEvent(string Operation, int Attempt, int? StatusCode, TimeSpan Duration, string Outcome);
