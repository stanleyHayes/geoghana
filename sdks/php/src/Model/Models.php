<?php

declare(strict_types=1);

namespace GhanaGeo\Model;

/** Exact immutable DTOs for contracts/openapi/v1.yaml. */
final readonly class Coordinate
{
    public function __construct(public float $latitude, public float $longitude) {}
}

final readonly class Ref
{
    public function __construct(public string $id, public string $name) {}
}

final readonly class Provenance
{
    public function __construct(public string $sourceId, public ?string $sourceUrl = null, public ?string $retrievedAt = null) {}
}

final readonly class Alias
{
    public function __construct(public string $value, public ?string $type = null, public ?string $language = null, public ?bool $isPreferred = null) {}
}

final readonly class Region
{
    public function __construct(
        public string $id, public string $countryCode, public string $name, public string $status,
        public string $verificationStatus, public Provenance $provenance, public string $datasetVersion,
        public ?string $capital = null, public ?string $code = null, public ?Coordinate $centroid = null,
    ) {}
}

final readonly class District
{
    public function __construct(
        public string $id, public string $name, public Ref $region, public string $status,
        public string $verificationStatus, public Provenance $provenance, public string $datasetVersion,
        public ?string $code = null, public ?string $type = null, public ?string $capital = null,
        public ?Coordinate $centroid = null,
    ) {}
}

class Place
{
    /** @param list<Alias> $aliases */
    public function __construct(
        public readonly string $id, public readonly string $name, public readonly string $normalizedName,
        public readonly string $type, public readonly array $aliases, public readonly string $status,
        public readonly string $verificationStatus, public readonly Provenance $provenance,
        public readonly string $datasetVersion, public readonly ?Ref $region = null,
        public readonly ?Ref $district = null, public readonly ?string $parentPlaceId = null,
        public readonly ?Coordinate $centroid = null, public readonly ?int $population = null,
    ) {}
}

final class SearchResult extends Place
{
    /** @param list<Alias> $aliases */
    public function __construct(
        string $id, string $name, string $normalizedName, string $type, array $aliases, string $status,
        string $verificationStatus, Provenance $provenance, string $datasetVersion, ?Ref $region = null,
        ?Ref $district = null, ?string $parentPlaceId = null, ?Coordinate $centroid = null,
        ?int $population = null, public readonly ?float $score = null, public readonly ?string $matchReason = null,
    ) {
        parent::__construct($id, $name, $normalizedName, $type, $aliases, $status, $verificationStatus, $provenance, $datasetVersion, $region, $district, $parentPlaceId, $centroid, $population);
    }
}

/** @template T */
final readonly class Page
{
    /** @param list<T> $data */
    public function __construct(public array $data, public string $datasetVersion, public ?string $nextCursor = null) {}
}

final readonly class Geometry
{
    /** @param list<mixed> $coordinates */
    public function __construct(public string $type, public array $coordinates) {}
}

final readonly class BoundaryProperties
{
    public function __construct(public ?string $id = null, public ?string $name = null, public ?string $datasetVersion = null, public ?string $attribution = null) {}
}

final readonly class BoundaryFeature
{
    public function __construct(public string $type, public Geometry $geometry, public BoundaryProperties $properties) {}
}

final readonly class ReverseResult
{
    /** @param list<Place>|null $nearby */
    public function __construct(public string $datasetVersion, public ?Ref $region = null, public ?Ref $district = null, public ?array $nearby = null) {}
}

final readonly class Dataset
{
    public function __construct(public string $version, public string $status, public ?string $publishedAt = null, public ?string $changelog = null, public ?string $checksum = null) {}
}

final readonly class DatasetPage
{
    /** @param list<Dataset> $data */
    public function __construct(public array $data) {}
}

final readonly class Download
{
    public function __construct(public string $format, public string $url, public string $checksum, public ?int $sizeBytes = null, public ?string $attribution = null) {}
}

final readonly class DownloadList
{
    /** @param list<Download> $downloads */
    public function __construct(public string $version, public array $downloads) {}
}
