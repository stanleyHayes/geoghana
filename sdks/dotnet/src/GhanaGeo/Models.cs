using System.Text.Json;
using System.Text.Json.Serialization;

namespace GhanaGeo;

public enum Status { Active, Deprecated, Merged }

public enum VerificationStatus { Reference, SeedNeedsCanonicalReconciliation, Reviewed, Canonical }

public enum PlaceType { City, Town, Village, Community, Suburb, Neighbourhood, Hamlet, Settlement, Locality, RegionalCapital }

public enum DatasetStatus { Draft, Validation, Review, Approved, Published, RolledBack }

public sealed record Coordinate(double Latitude, double Longitude);
public sealed record GeoRef(string Id, string Name);
public sealed record Provenance(string SourceId, string? SourceUrl = null, string? RetrievedAt = null);
#pragma warning disable CA1716 // Mirrors the contract's Alias schema.
public sealed record Alias(string Value, string? Type = null, string? Language = null, bool? IsPreferred = null);
#pragma warning restore CA1716

public sealed record Region(
    string Id, string CountryCode, string Name, Status Status,
    VerificationStatus VerificationStatus, Provenance Provenance, string DatasetVersion,
    string? Capital = null, string? Code = null, Coordinate? Centroid = null);

public sealed record District(
    string Id, string Name, GeoRef Region, Status Status,
    VerificationStatus VerificationStatus, Provenance Provenance, string DatasetVersion,
    string? Code = null, string? Type = null, string? Capital = null, Coordinate? Centroid = null);

public record Place(
    string Id, string Name, string NormalizedName, PlaceType Type, IReadOnlyList<Alias> Aliases,
    Status Status, VerificationStatus VerificationStatus, Provenance Provenance, string DatasetVersion,
    GeoRef? Region = null, GeoRef? District = null, string? ParentPlaceId = null,
    Coordinate? Centroid = null, long? Population = null);

public sealed record SearchResult(
    string Id, string Name, string NormalizedName, PlaceType Type, IReadOnlyList<Alias> Aliases,
    Status Status, VerificationStatus VerificationStatus, Provenance Provenance, string DatasetVersion,
    GeoRef? Region = null, GeoRef? District = null, string? ParentPlaceId = null,
    Coordinate? Centroid = null, long? Population = null, double? Score = null, string? MatchReason = null)
    : Place(Id, Name, NormalizedName, Type, Aliases, Status, VerificationStatus, Provenance,
        DatasetVersion, Region, District, ParentPlaceId, Centroid, Population);

public sealed record Page<T>(IReadOnlyList<T> Data, string DatasetVersion, string? NextCursor = null);
public sealed record ReverseResult(string DatasetVersion, GeoRef? Region = null, GeoRef? District = null, IReadOnlyList<Place>? Nearby = null);
public sealed record GeoJsonGeometry(string Type, JsonElement Coordinates);
public sealed record BoundaryProperties(string? Id = null, string? Name = null, string? DatasetVersion = null, string? Attribution = null);
public sealed record BoundaryFeature(string Type, GeoJsonGeometry Geometry, BoundaryProperties Properties);
public sealed record DatasetVersion(string Version, DatasetStatus Status, DateTimeOffset? PublishedAt = null, string? Changelog = null, string? Checksum = null);
public sealed record DatasetPage(IReadOnlyList<DatasetVersion> Data);
public sealed record DatasetDownload(string Format, Uri Url, string Checksum, long? SizeBytes = null, string? Attribution = null);
public sealed record DownloadList(string Version, IReadOnlyList<DatasetDownload> Downloads);

public sealed record ListOptions(string? Cursor = null, int? Limit = null);
public sealed record DistrictListOptions(string? Cursor = null, int? Limit = null, string? RegionId = null, string? Query = null);
public sealed record PlaceListOptions(string? Cursor = null, int? Limit = null, string? DistrictId = null, string? RegionId = null, PlaceType? Type = null, string? Query = null);
public sealed record SearchOptions(string Query, int? Limit = null, string? RegionId = null, string? DistrictId = null, PlaceType? Type = null);
public sealed record TextQueryOptions(string Query, int? Limit = null);
public sealed record NearbyOptions(double Latitude, double Longitude, double Radius, int? Limit = null);
public sealed record DatasetArtifactOptions(long MaximumBytes = 268_435_456, string? ExpectedSha256 = null);
