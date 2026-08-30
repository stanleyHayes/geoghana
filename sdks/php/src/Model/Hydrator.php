<?php

declare(strict_types=1);

namespace GhanaGeo\Model;

final class Hydrator
{
    /** @param array<string, mixed> $v */
    public static function region(array $v): Region
    {
        return new Region(self::string($v, 'id'), self::string($v, 'countryCode'), self::string($v, 'name'), self::string($v, 'status'), self::string($v, 'verificationStatus'), self::provenance(self::array($v, 'provenance')), self::string($v, 'datasetVersion'), self::nullableString($v, 'capital'), self::nullableString($v, 'code'), self::coordinate($v['centroid'] ?? null));
    }

    /** @param array<string, mixed> $v */
    public static function district(array $v): District
    {
        return new District(self::string($v, 'id'), self::string($v, 'name'), self::ref(self::array($v, 'region')), self::string($v, 'status'), self::string($v, 'verificationStatus'), self::provenance(self::array($v, 'provenance')), self::string($v, 'datasetVersion'), self::nullableString($v, 'code'), self::nullableString($v, 'type'), self::nullableString($v, 'capital'), self::coordinate($v['centroid'] ?? null));
    }

    /** @param array<string, mixed> $v */
    public static function place(array $v, bool $search = false): Place
    {
        $args = [self::string($v, 'id'), self::string($v, 'name'), self::string($v, 'normalizedName'), self::string($v, 'type'), array_map(self::alias(...), self::list($v, 'aliases')), self::string($v, 'status'), self::string($v, 'verificationStatus'), self::provenance(self::array($v, 'provenance')), self::string($v, 'datasetVersion'), isset($v['region']) ? self::ref(self::array($v, 'region')) : null, isset($v['district']) ? self::ref(self::array($v, 'district')) : null, self::nullableString($v, 'parentPlaceId'), self::coordinate($v['centroid'] ?? null), isset($v['population']) ? (int) $v['population'] : null];

        return $search ? new SearchResult(...[...$args, isset($v['score']) ? (float) $v['score'] : null, self::nullableString($v, 'matchReason')]) : new Place(...$args);
    }

    /** @param array<string, mixed> $v */
    public static function searchResult(array $v): SearchResult
    {
        $place = self::place($v, true);
        if (! $place instanceof SearchResult) {
            throw new \LogicException('Search hydration failed.');
        }

        return $place;
    }

    /** @param array<string, mixed> $v @param callable(array<string,mixed>): mixed $item */
    /**
     * @template T
     *
     * @param  array<string, mixed>  $v
     * @param  callable(array<string,mixed>): T  $item
     * @return Page<T>
     */
    public static function page(array $v, callable $item): Page
    {
        return new Page(array_map($item, self::list($v, 'data')), self::string($v, 'datasetVersion'), self::nullableString($v, 'nextCursor'));
    }

    /** @param array<string, mixed> $v */
    public static function reverse(array $v): ReverseResult
    {
        return new ReverseResult(self::string($v, 'datasetVersion'), isset($v['region']) ? self::ref(self::array($v, 'region')) : null, isset($v['district']) ? self::ref(self::array($v, 'district')) : null, isset($v['nearby']) ? array_map(self::place(...), self::list($v, 'nearby')) : null);
    }

    /** @param array<string, mixed> $v */
    public static function boundary(array $v): BoundaryFeature
    {
        $g = self::array($v, 'geometry');
        $p = self::array($v, 'properties');

        return new BoundaryFeature(self::string($v, 'type'), new Geometry(self::string($g, 'type'), self::list($g, 'coordinates')), new BoundaryProperties(self::nullableString($p, 'id'), self::nullableString($p, 'name'), self::nullableString($p, 'datasetVersion'), self::nullableString($p, 'attribution')));
    }

    /** @param array<string, mixed> $v */
    public static function dataset(array $v): Dataset
    {
        return new Dataset(self::string($v, 'version'), self::string($v, 'status'), self::nullableString($v, 'publishedAt'), self::nullableString($v, 'changelog'), self::nullableString($v, 'checksum'));
    }

    /** @param array<string, mixed> $v */
    public static function download(array $v): Download
    {
        return new Download(self::string($v, 'format'), self::string($v, 'url'), self::string($v, 'checksum'), isset($v['sizeBytes']) ? (int) $v['sizeBytes'] : null, self::nullableString($v, 'attribution'));
    }

    /** @param array<string, mixed> $v */
    private static function provenance(array $v): Provenance
    {
        return new Provenance(self::string($v, 'sourceId'), self::nullableString($v, 'sourceUrl'), self::nullableString($v, 'retrievedAt'));
    }

    /** @param array<string, mixed> $v */
    private static function ref(array $v): Ref
    {
        return new Ref(self::string($v, 'id'), self::string($v, 'name'));
    }

    /** @param array<string, mixed> $v */
    private static function alias(array $v): Alias
    {
        return new Alias(self::string($v, 'value'), self::nullableString($v, 'type'), self::nullableString($v, 'language'), isset($v['isPreferred']) ? (bool) $v['isPreferred'] : null);
    }

    private static function coordinate(mixed $v): ?Coordinate
    {
        return is_array($v) ? new Coordinate((float) ($v['latitude'] ?? 0), (float) ($v['longitude'] ?? 0)) : null;
    }

    /** @param array<string, mixed> $v */
    private static function string(array $v, string $key): string
    {
        if (! isset($v[$key]) || ! is_string($v[$key])) {
            throw new \UnexpectedValueException("Missing string field {$key}");
        }

        return $v[$key];
    }

    /** @param array<string, mixed> $v */
    private static function nullableString(array $v, string $key): ?string
    {
        return isset($v[$key]) && is_string($v[$key]) ? $v[$key] : null;
    }

    /**
     * @param  array<string, mixed>  $v
     * @return array<string, mixed>
     */
    private static function array(array $v, string $key): array
    {
        if (! isset($v[$key]) || ! is_array($v[$key])) {
            throw new \UnexpectedValueException("Missing object field {$key}");
        }

        return $v[$key];
    }

    /**
     * @param  array<string, mixed>  $v
     * @return list<mixed>
     */
    private static function list(array $v, string $key): array
    {
        $out = $v[$key] ?? [];
        if (! is_array($out) || ! array_is_list($out)) {
            throw new \UnexpectedValueException("Invalid list field {$key}");
        }

        return $out;
    }
}
