package dev.ghanageo;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.databind.JsonNode;
import java.util.List;
import java.util.Map;

/** Immutable types generated from the locked GhanaGeo OpenAPI geography, search and dataset schemas. */
public final class Models {
  private Models() {}
  public enum Status { ACTIVE, DEPRECATED, MERGED }
  public enum VerificationStatus { REFERENCE, SEED_NEEDS_CANONICAL_RECONCILIATION, REVIEWED, CANONICAL }
  public enum PlaceType { CITY, TOWN, VILLAGE, COMMUNITY, SUBURB, NEIGHBOURHOOD, HAMLET, SETTLEMENT, LOCALITY, REGIONAL_CAPITAL }
  public enum DatasetStatus { draft, validation, review, approved, published, rolled_back }
  public enum DownloadFormat { json, csv, geojson, parquet }
  @JsonIgnoreProperties(ignoreUnknown=true) public record Coordinate(double latitude,double longitude) {}
  @JsonIgnoreProperties(ignoreUnknown=true) public record Ref(String id,String name) {}
  @JsonIgnoreProperties(ignoreUnknown=true) public record Provenance(String sourceId,String sourceUrl,String retrievedAt) {}
  @JsonIgnoreProperties(ignoreUnknown=true) public record Alias(String value,String type,String language,Boolean isPreferred) {}
  @JsonIgnoreProperties(ignoreUnknown=true) public record Region(String id,String countryCode,String name,String capital,String code,Status status,VerificationStatus verificationStatus,Coordinate centroid,Provenance provenance,String datasetVersion) {}
  @JsonIgnoreProperties(ignoreUnknown=true) public record District(String id,String name,String code,String type,String capital,Ref region,Status status,VerificationStatus verificationStatus,Coordinate centroid,Provenance provenance,String datasetVersion) {}
  @JsonIgnoreProperties(ignoreUnknown=true) public record Place(String id,String name,String normalizedName,PlaceType type,Ref region,Ref district,String parentPlaceId,List<Alias> aliases,Coordinate centroid,Long population,Status status,VerificationStatus verificationStatus,Provenance provenance,String datasetVersion) { public Place { aliases=aliases==null?List.of():List.copyOf(aliases); } }
  @JsonIgnoreProperties(ignoreUnknown=true) public record SearchResult(String id,String name,String normalizedName,PlaceType type,Ref region,Ref district,String parentPlaceId,List<Alias> aliases,Coordinate centroid,Long population,Status status,VerificationStatus verificationStatus,Provenance provenance,String datasetVersion,Double score,String matchReason) { public SearchResult { aliases=aliases==null?List.of():List.copyOf(aliases); } }
  public interface Page<T> { List<T> data(); String datasetVersion(); String nextCursor(); }
  @JsonIgnoreProperties(ignoreUnknown=true) public record RegionPage(List<Region> data,String datasetVersion,String nextCursor) implements Page<Region> { public RegionPage { data=data==null?List.of():List.copyOf(data); } }
  @JsonIgnoreProperties(ignoreUnknown=true) public record DistrictPage(List<District> data,String datasetVersion,String nextCursor) implements Page<District> { public DistrictPage { data=data==null?List.of():List.copyOf(data); } }
  @JsonIgnoreProperties(ignoreUnknown=true) public record PlacePage(List<Place> data,String datasetVersion,String nextCursor) implements Page<Place> { public PlacePage { data=data==null?List.of():List.copyOf(data); } }
  @JsonIgnoreProperties(ignoreUnknown=true) public record SearchPage(List<SearchResult> data,String datasetVersion,String nextCursor) implements Page<SearchResult> { public SearchPage { data=data==null?List.of():List.copyOf(data); } }
  @JsonIgnoreProperties(ignoreUnknown=true) public record ReverseResult(Ref region,Ref district,List<Place> nearby,String datasetVersion) { public ReverseResult { nearby=nearby==null?List.of():List.copyOf(nearby); } }
  @JsonIgnoreProperties(ignoreUnknown=true) public record Geometry(String type,JsonNode coordinates) {}
  @JsonIgnoreProperties(ignoreUnknown=true) public record BoundaryProperties(String id,String name,String datasetVersion,String attribution) {}
  @JsonIgnoreProperties(ignoreUnknown=true) public record BoundaryFeature(String type,Geometry geometry,BoundaryProperties properties) {}
  @JsonIgnoreProperties(ignoreUnknown=true) public record DatasetVersion(String version,DatasetStatus status,String publishedAt,String changelog,String checksum) {}
  @JsonIgnoreProperties(ignoreUnknown=true) public record DatasetPage(List<DatasetVersion> data) { public DatasetPage { data=data==null?List.of():List.copyOf(data); } }
  @JsonIgnoreProperties(ignoreUnknown=true) public record Download(DownloadFormat format,String url,Long sizeBytes,String checksum,String attribution) {}
  @JsonIgnoreProperties(ignoreUnknown=true) public record DownloadList(String version,List<Download> downloads) { public DownloadList { downloads=downloads==null?List.of():List.copyOf(downloads); } }
  public record OpenData(Map<String,JsonNode> fields) { public OpenData { fields=Map.copyOf(fields); } }
  public record ErrorBody(String code,String message,String requestId,Map<String,JsonNode> details,String docs) { public ErrorBody { details=details==null?Map.of():Map.copyOf(details); } }
  public record TelemetryEvent(String path,String method,int attempt,Integer status,long durationMillis,String errorCode) {}
  public record WireEvidence(String phase,String method,String uri,int attempt,Integer status,Map<String,List<String>> headers,long bodyBytes,String bodySha256,String redactedPreview) { public WireEvidence { headers=Map.copyOf(headers); } }
}
