typedef JsonMap = Map<String, Object?>;

T _enumValue<T extends Enum>(List<T> values, Object? wire, String field) {
  final normalizedWire =
      wire?.toString().replaceAll(RegExp('[^A-Za-z0-9]'), '').toLowerCase();
  for (final value in values) {
    final normalizedName =
        value.name.replaceAll(RegExp('[^A-Za-z0-9]'), '').toLowerCase();
    if (normalizedName == normalizedWire) return value;
  }
  throw FormatException('Unknown $field value: $wire');
}

enum Status { active, deprecated, merged }

enum VerificationStatus {
  reference,
  seedNeedsCanonicalReconciliation,
  reviewed,
  canonical
}

enum PlaceType {
  city,
  town,
  village,
  community,
  suburb,
  neighbourhood,
  hamlet,
  settlement,
  locality,
  regionalCapital
}

enum DatasetStatus {
  draft,
  validation,
  review,
  approved,
  published,
  rolledBack
}

enum DownloadFormat { json, csv, geojson, parquet }

final class Coordinate {
  const Coordinate({required this.latitude, required this.longitude});
  factory Coordinate.fromJson(JsonMap json) => Coordinate(
        latitude: (json['latitude']! as num).toDouble(),
        longitude: (json['longitude']! as num).toDouble(),
      );
  final double latitude;
  final double longitude;
}

final class GeoReference {
  const GeoReference({required this.id, required this.name});
  factory GeoReference.fromJson(JsonMap json) =>
      GeoReference(id: json['id']! as String, name: json['name']! as String);
  final String id;
  final String name;
}

final class Provenance {
  const Provenance({required this.sourceId, this.sourceUrl, this.retrievedAt});
  factory Provenance.fromJson(JsonMap json) => Provenance(
        sourceId: json['sourceId']! as String,
        sourceUrl: json['sourceUrl'] == null
            ? null
            : Uri.parse(json['sourceUrl']! as String),
        retrievedAt: json['retrievedAt'] as String?,
      );
  final String sourceId;
  final Uri? sourceUrl;
  final String? retrievedAt;
}

final class Alias {
  const Alias(
      {required this.value, this.type, this.language, this.isPreferred});
  factory Alias.fromJson(JsonMap json) => Alias(
        value: json['value']! as String,
        type: json['type'] as String?,
        language: json['language'] as String?,
        isPreferred: json['isPreferred'] as bool?,
      );
  final String value;
  final String? type;
  final String? language;
  final bool? isPreferred;
}

final class Page<T> {
  const Page(
      {required this.data, required this.datasetVersion, this.nextCursor});
  final List<T> data;
  final String datasetVersion;
  final String? nextCursor;
}

final class Region {
  const Region({
    required this.id,
    required this.countryCode,
    required this.name,
    required this.status,
    required this.verificationStatus,
    required this.provenance,
    required this.datasetVersion,
    this.capital,
    this.code,
    this.centroid,
    this.raw = const {},
  });
  factory Region.fromJson(JsonMap json) => Region(
        id: json['id']! as String,
        countryCode: json['countryCode']! as String,
        name: json['name']! as String,
        capital: json['capital'] as String?,
        code: json['code'] as String?,
        status: _enumValue(Status.values, json['status'], 'status'),
        verificationStatus: _enumValue(VerificationStatus.values,
            _camelEnum(json['verificationStatus']), 'verificationStatus'),
        centroid: _object(json['centroid'], Coordinate.fromJson),
        provenance: Provenance.fromJson(_json(json['provenance'])),
        datasetVersion: json['datasetVersion']! as String,
        raw: json,
      );
  final String id;
  final String countryCode;
  final String name;
  final String? capital;
  final String? code;
  final Status status;
  final VerificationStatus verificationStatus;
  final Coordinate? centroid;
  final Provenance provenance;
  final String datasetVersion;
  final JsonMap raw;
}

final class District {
  const District({
    required this.id,
    required this.name,
    required this.region,
    required this.status,
    required this.verificationStatus,
    required this.provenance,
    required this.datasetVersion,
    this.code,
    this.type,
    this.capital,
    this.centroid,
    this.raw = const {},
  });
  factory District.fromJson(JsonMap json) => District(
        id: json['id']! as String,
        name: json['name']! as String,
        code: json['code'] as String?,
        type: json['type'] as String?,
        capital: json['capital'] as String?,
        region: GeoReference.fromJson(_json(json['region'])),
        status: _enumValue(Status.values, json['status'], 'status'),
        verificationStatus: _enumValue(VerificationStatus.values,
            _camelEnum(json['verificationStatus']), 'verificationStatus'),
        centroid: _object(json['centroid'], Coordinate.fromJson),
        provenance: Provenance.fromJson(_json(json['provenance'])),
        datasetVersion: json['datasetVersion']! as String,
        raw: json,
      );
  final String id;
  final String name;
  final String? code;
  final String? type;
  final String? capital;
  final GeoReference region;
  final Status status;
  final VerificationStatus verificationStatus;
  final Coordinate? centroid;
  final Provenance provenance;
  final String datasetVersion;
  final JsonMap raw;
}

class Place {
  const Place({
    required this.id,
    required this.name,
    required this.normalizedName,
    required this.type,
    required this.aliases,
    required this.status,
    required this.verificationStatus,
    required this.provenance,
    required this.datasetVersion,
    this.region,
    this.district,
    this.parentPlaceId,
    this.centroid,
    this.population,
    this.raw = const {},
  });
  factory Place.fromJson(JsonMap json) =>
      Place._from(_PlaceFields.fromJson(json), json);
  Place._from(_PlaceFields f, JsonMap raw)
      : this(
          id: f.id,
          name: f.name,
          normalizedName: f.normalizedName,
          type: f.type,
          aliases: f.aliases,
          status: f.status,
          verificationStatus: f.verificationStatus,
          provenance: f.provenance,
          datasetVersion: f.datasetVersion,
          region: f.region,
          district: f.district,
          parentPlaceId: f.parentPlaceId,
          centroid: f.centroid,
          population: f.population,
          raw: raw,
        );
  final String id;
  final String name;
  final String normalizedName;
  final PlaceType type;
  final GeoReference? region;
  final GeoReference? district;
  final String? parentPlaceId;
  final List<Alias> aliases;
  final Coordinate? centroid;
  final int? population;
  final Status status;
  final VerificationStatus verificationStatus;
  final Provenance provenance;
  final String datasetVersion;
  final JsonMap raw;
}

final class SearchResult extends Place {
  // The private parsing carrier keeps the public constructor fully typed.
  // ignore: use_super_parameters
  SearchResult._(_PlaceFields fields, JsonMap raw,
      {required this.score, required this.matchReason})
      : super._from(fields, raw);
  factory SearchResult.fromJson(JsonMap json) {
    final source =
        json['place'] is Map<Object?, Object?> ? _json(json['place']) : json;
    return SearchResult._(
      _PlaceFields.fromJson(source),
      source,
      score: (json['score'] as num?)?.toDouble(),
      matchReason: json['matchReason'] as String?,
    );
  }
  final double? score;
  final String? matchReason;
  Place get place => this;
}

final class ReverseResult {
  const ReverseResult(
      {required this.nearby,
      required this.datasetVersion,
      this.region,
      this.district});
  factory ReverseResult.fromJson(JsonMap json) => ReverseResult(
        region: _object(json['region'], GeoReference.fromJson),
        district: _object(json['district'], GeoReference.fromJson),
        nearby: _list(json['nearby'], Place.fromJson),
        datasetVersion: json['datasetVersion']! as String,
      );
  final GeoReference? region;
  final GeoReference? district;
  final List<Place> nearby;
  final String datasetVersion;
}

final class GeoJsonGeometry {
  const GeoJsonGeometry({required this.type, required this.coordinates});
  factory GeoJsonGeometry.fromJson(JsonMap json) => GeoJsonGeometry(
      type: json['type']! as String,
      coordinates: json['coordinates']! as List<Object?>);
  final String type;
  final List<Object?> coordinates;
}

final class BoundaryProperties {
  const BoundaryProperties(
      {this.id, this.name, this.datasetVersion, this.attribution});
  factory BoundaryProperties.fromJson(JsonMap json) => BoundaryProperties(
      id: json['id'] as String?,
      name: json['name'] as String?,
      datasetVersion: json['datasetVersion'] as String?,
      attribution: json['attribution'] as String?);
  final String? id;
  final String? name;
  final String? datasetVersion;
  final String? attribution;
}

final class BoundaryFeature {
  const BoundaryFeature({required this.geometry, required this.properties});
  factory BoundaryFeature.fromJson(JsonMap json) {
    if (json['type'] != 'Feature') {
      throw const FormatException('Boundary type must be Feature');
    }
    return BoundaryFeature(
        geometry: GeoJsonGeometry.fromJson(_json(json['geometry'])),
        properties: BoundaryProperties.fromJson(_json(json['properties'])));
  }
  String get type => 'Feature';
  final GeoJsonGeometry geometry;
  final BoundaryProperties properties;
}

final class DatasetVersion {
  const DatasetVersion(
      {required this.version,
      required this.status,
      this.publishedAt,
      this.changelog,
      this.checksum});
  factory DatasetVersion.fromJson(JsonMap json) => DatasetVersion(
        version: json['version']! as String,
        status: _enumValue(
            DatasetStatus.values, _camelEnum(json['status']), 'dataset status'),
        publishedAt: json['publishedAt'] == null
            ? null
            : DateTime.parse(json['publishedAt']! as String),
        changelog: json['changelog'] as String?,
        checksum: json['checksum'] as String?,
      );
  final String version;
  final DatasetStatus status;
  final DateTime? publishedAt;
  final String? changelog;
  final String? checksum;
}

final class DatasetPage {
  const DatasetPage({required this.data});
  factory DatasetPage.fromJson(JsonMap json) =>
      DatasetPage(data: _list(json['data'], DatasetVersion.fromJson));
  final List<DatasetVersion> data;
}

final class DatasetDownload {
  const DatasetDownload(
      {required this.format,
      required this.url,
      required this.checksum,
      this.sizeBytes,
      this.attribution});
  factory DatasetDownload.fromJson(JsonMap json) => DatasetDownload(
        format: _enumValue(
            DownloadFormat.values, json['format'], 'download format'),
        url: Uri.parse(json['url']! as String),
        checksum: json['checksum']! as String,
        sizeBytes: (json['sizeBytes'] as num?)?.toInt(),
        attribution: json['attribution'] as String?,
      );
  final DownloadFormat format;
  final Uri url;
  final String checksum;
  final int? sizeBytes;
  final String? attribution;
}

final class DownloadList {
  const DownloadList({required this.version, required this.downloads});
  factory DownloadList.fromJson(JsonMap json) => DownloadList(
      version: json['version']! as String,
      downloads: _list(json['downloads'], DatasetDownload.fromJson));
  final String version;
  final List<DatasetDownload> downloads;
}

final class DatasetChange {
  const DatasetChange({required this.change, this.cursor});
  factory DatasetChange.fromJson(JsonMap json) => DatasetChange(
      change: _json(json['change']), cursor: json['cursor'] as String?);
  final JsonMap change;
  final String? cursor;
}

final class DatasetArtifact {
  const DatasetArtifact({required this.bytes, required this.contentType});
  final List<int> bytes;
  final String? contentType;
}

final class _PlaceFields {
  const _PlaceFields(
      {required this.id,
      required this.name,
      required this.normalizedName,
      required this.type,
      required this.aliases,
      required this.status,
      required this.verificationStatus,
      required this.provenance,
      required this.datasetVersion,
      this.region,
      this.district,
      this.parentPlaceId,
      this.centroid,
      this.population});
  factory _PlaceFields.fromJson(JsonMap json) => _PlaceFields(
        id: json['id']! as String,
        name: json['name']! as String,
        normalizedName: json['normalizedName']! as String,
        type: _enumValue(
            PlaceType.values, _camelEnum(json['type']), 'place type'),
        region: _object(json['region'], GeoReference.fromJson),
        district: _object(json['district'], GeoReference.fromJson),
        parentPlaceId: json['parentPlaceId'] as String?,
        aliases: _list(json['aliases'], Alias.fromJson),
        centroid: _object(json['centroid'], Coordinate.fromJson),
        population: (json['population'] as num?)?.toInt(),
        status: _enumValue(Status.values, json['status'], 'status'),
        verificationStatus: _enumValue(VerificationStatus.values,
            _camelEnum(json['verificationStatus']), 'verificationStatus'),
        provenance: Provenance.fromJson(_json(json['provenance'])),
        datasetVersion: json['datasetVersion']! as String,
      );
  final String id;
  final String name;
  final String normalizedName;
  final PlaceType type;
  final GeoReference? region;
  final GeoReference? district;
  final String? parentPlaceId;
  final List<Alias> aliases;
  final Coordinate? centroid;
  final int? population;
  final Status status;
  final VerificationStatus verificationStatus;
  final Provenance provenance;
  final String datasetVersion;
}

String? _camelEnum(Object? wire) => switch (wire) {
      'SEED_NEEDS_CANONICAL_RECONCILIATION' =>
        'SEEDNEEDSCANONICALRECONCILIATION',
      'REGIONAL_CAPITAL' => 'REGIONALCAPITAL',
      'rolled_back' => 'ROLLEDBACK',
      String value => value.replaceAll('_', '').toUpperCase(),
      _ => null,
    };
JsonMap _json(Object? value) =>
    Map<String, Object?>.from(value! as Map<Object?, Object?>);
T? _object<T>(Object? value, T Function(JsonMap) decode) =>
    value == null ? null : decode(_json(value));
List<T> _list<T>(Object? value, T Function(JsonMap) decode) =>
    (value! as List<Object?>)
        .map((item) => decode(_json(item)))
        .toList(growable: false);
