<?php

declare(strict_types=1);

namespace GhanaGeo;

use GhanaGeo\Exception\DownloadException;
use GhanaGeo\Exception\GhanaGeoException;
use GhanaGeo\Exception\TransportException;
use GhanaGeo\Model\BoundaryFeature;
use GhanaGeo\Model\DatasetPage;
use GhanaGeo\Model\District;
use GhanaGeo\Model\DownloadList;
use GhanaGeo\Model\Hydrator;
use GhanaGeo\Model\Page;
use GhanaGeo\Model\Place;
use GhanaGeo\Model\Region;
use GhanaGeo\Model\ReverseResult;
use GhanaGeo\Model\SearchResult;
use GhanaGeo\Transport\DeadlineAwareClientInterface;
use Psr\Http\Client\ClientExceptionInterface;
use Psr\Http\Client\ClientInterface;
use Psr\Http\Message\RequestFactoryInterface;
use Psr\Http\Message\ResponseInterface;
use Psr\Log\LoggerInterface;
use Psr\Log\NullLogger;

final class Client
{
    public const DEFAULT_BASE_URL = 'https://api.geo.digitalghana.dev/v1';

    public const DEFAULT_MAX_BODY_BYTES = 8_388_608;

    public const DEFAULT_MAX_DOWNLOAD_BYTES = 268_435_456;

    /** @var null|callable(array<string, scalar|null>): void */
    private $telemetry;

    /**
     * Cancellation is delegated to the injected PSR-18 implementation: PSR-18 has no
     * portable cancellation token. Configure deadlines/cancellation on that client.
     *
     * @param  null|callable(array<string, scalar|null>): void  $telemetry  Disabled by default.
     */
    public function __construct(
        private readonly ClientInterface $http,
        private readonly RequestFactoryInterface $requests,
        private readonly string $baseUrl = self::DEFAULT_BASE_URL,
        private readonly ?string $apiKey = null,
        private readonly ?RetryPolicy $retry = new RetryPolicy,
        ?LoggerInterface $logger = null,
        ?callable $telemetry = null,
        private readonly int $maxBodyBytes = self::DEFAULT_MAX_BODY_BYTES,
        private readonly int $maxDownloadBytes = self::DEFAULT_MAX_DOWNLOAD_BYTES,
        private readonly ?float $defaultTimeoutSeconds = null,
        /** @var array<string, string> */ private readonly array $headers = [],
    ) {
        if ($maxBodyBytes < 1 || $maxDownloadBytes < 1) {
            throw new \InvalidArgumentException('Body limits must be positive.');
        }
        if ($defaultTimeoutSeconds !== null && (! is_finite($defaultTimeoutSeconds) || $defaultTimeoutSeconds <= 0)) {
            throw new \InvalidArgumentException('Default timeout must be a finite positive number.');
        }
        $this->logger = $logger ?? new NullLogger;
        $this->telemetry = $telemetry;
    }

    private readonly LoggerInterface $logger;

    public function apiVersion(): string
    {
        return Version::API;
    }

    public function testedDatasetVersion(): string
    {
        return Version::TESTED_DATASET;
    }

    public function sdkVersion(): string
    {
        return Version::SDK;
    }

    public function region(string $id): Region
    {
        return Hydrator::region($this->json('/regions/'.self::segment($id)));
    }

    /** @return Page<Region> */
    public function regions(?string $cursor = null, ?int $limit = null): Page
    {
        return Hydrator::page($this->json('/regions', compact('cursor', 'limit')), Hydrator::region(...));
    }

    public function district(string $id): District
    {
        return Hydrator::district($this->json('/districts/'.self::segment($id)));
    }

    /** @return Page<District> */
    public function regionDistricts(string $id, ?string $cursor = null, ?int $limit = null): Page
    {
        return Hydrator::page($this->json('/regions/'.self::segment($id).'/districts', compact('cursor', 'limit')), Hydrator::district(...));
    }

    /** @return Page<District> */
    public function districts(?string $regionId = null, ?string $q = null, ?string $cursor = null, ?int $limit = null): Page
    {
        return Hydrator::page($this->json('/districts', compact('regionId', 'q', 'cursor', 'limit')), Hydrator::district(...));
    }

    public function place(string $id): Place
    {
        return Hydrator::place($this->json('/places/'.self::segment($id)));
    }

    /** @return Page<Place> */
    public function districtPlaces(string $id, ?string $cursor = null, ?int $limit = null, ?string $type = null): Page
    {
        return Hydrator::page($this->json('/districts/'.self::segment($id).'/places', compact('cursor', 'limit', 'type')), Hydrator::place(...));
    }

    /** @return Page<Place> */
    public function places(?string $districtId = null, ?string $regionId = null, ?string $type = null, ?string $q = null, ?string $cursor = null, ?int $limit = null): Page
    {
        return Hydrator::page($this->json('/places', compact('districtId', 'regionId', 'type', 'q', 'cursor', 'limit')), Hydrator::place(...));
    }

    /** @return Page<SearchResult> */
    public function search(string $q, ?string $regionId = null, ?string $districtId = null, ?string $type = null, ?int $limit = null, ?RequestOptions $options = null): Page
    {
        return Hydrator::page($this->json('/search', compact('q', 'regionId', 'districtId', 'type', 'limit'), options: $options), Hydrator::searchResult(...));
    }

    /** @return Page<SearchResult> */
    public function autocomplete(string $q, ?int $limit = null): Page
    {
        return Hydrator::page($this->json('/autocomplete', compact('q', 'limit')), Hydrator::searchResult(...));
    }

    /** @return Page<SearchResult> */
    public function geocode(string $q, ?int $limit = null): Page
    {
        return Hydrator::page($this->json('/geocode', compact('q', 'limit')), Hydrator::searchResult(...));
    }

    public function reverseGeocode(float $lat, float $lng): ReverseResult
    {
        return Hydrator::reverse($this->json('/reverse', compact('lat', 'lng')));
    }

    /** @return Page<Place> */
    public function nearby(float $lat, float $lng, int $radius = 5000, int $limit = 20): Page
    {
        return Hydrator::page($this->json('/nearby', compact('lat', 'lng', 'radius', 'limit')), Hydrator::place(...));
    }

    public function boundary(string $id): BoundaryFeature
    {
        return Hydrator::boundary($this->json('/boundaries/'.self::segment($id), accept: 'application/geo+json'));
    }

    public function datasets(): DatasetPage
    {
        $value = $this->json('/datasets');

        return new DatasetPage(array_values(array_map(Hydrator::dataset(...), $value['data'])));
    }

    public function datasetDownloads(string $version): DownloadList
    {
        $v = $this->json('/datasets/'.self::segment($version).'/downloads');

        return new DownloadList((string) $v['version'], array_values(array_map(Hydrator::download(...), $v['downloads'])));
    }

    /** @return array<string, mixed> */
    public function roads(): array
    {
        return $this->json('/roads');
    }

    /** @return array<string, mixed> */
    public function pointsOfInterest(): array
    {
        return $this->json('/pois');
    }

    /** @return \Generator<int, Page<mixed>> */
    public function pages(callable $fetch): \Generator
    {
        $cursor = null;
        $seen = [];
        do {
            $page = $fetch($cursor);
            if (! $page instanceof Page) {
                throw new \UnexpectedValueException('Paginator callback must return Page.');
            }
            yield $page;
            $cursor = $page->nextCursor;
            if ($cursor !== null && isset($seen[$cursor])) {
                throw new GhanaGeoException('Repeated pagination cursor.', 'INVALID_CURSOR');
            }
            if ($cursor !== null) {
                $seen[$cursor] = true;
            }
        } while ($cursor !== null);
    }

    /** @return \Generator<int, Region> */
    public function allRegions(?int $limit = null): \Generator
    {
        foreach ($this->pages(fn (?string $c): Page => $this->regions($c, $limit)) as $page) {
            foreach ($page->data as $item) {
                yield $item;
            }
        }
    }

    /** @return \Generator<int, District> */
    public function allDistricts(?string $regionId = null, ?string $q = null, ?int $limit = null): \Generator
    {
        foreach ($this->pages(fn (?string $c): Page => $this->districts($regionId, $q, $c, $limit)) as $page) {
            foreach ($page->data as $item) {
                yield $item;
            }
        }
    }

    /** @return \Generator<int, Place> */
    public function allPlaces(?string $districtId = null, ?string $regionId = null, ?string $type = null, ?string $q = null, ?int $limit = null): \Generator
    {
        foreach ($this->pages(fn (?string $c): Page => $this->places($districtId, $regionId, $type, $q, $c, $limit)) as $page) {
            foreach ($page->data as $item) {
                yield $item;
            }
        }
    }

    public function downloadDatasetArtifact(string $version, string $entity, string $format, ?string $destination = null, ?string $checksum = null): string
    {
        $response = $this->send('/datasets/'.self::segment($version).'/downloads/'.self::segment($entity).'.'.self::segment($format), 'application/octet-stream');
        $length = $response->getHeaderLine('Content-Length');
        if ($length !== '' && (int) $length > $this->maxDownloadBytes) {
            $response->getBody()->close();
            throw new DownloadException('Download exceeds configured limit.', 'PAYLOAD_TOO_LARGE', $response->getStatusCode());
        }

        return $this->consumeDownload($response, $destination, $checksum);
    }

    /**
     * @param  array<string, scalar|null>  $query
     * @return array<string, mixed>
     */
    public function raw(string $path, array $query = []): array
    {
        return $this->json($path, $query);
    }

    /**
     * @param  array<string, scalar|null>  $query
     * @return array<string, mixed>
     */
    private function json(string $path, array $query = [], string $accept = 'application/json', ?RequestOptions $options = null): array
    {
        $response = $this->send($path, $accept, $query, $options);
        $body = $this->boundedBody($response, $this->maxBodyBytes);
        try {
            $decoded = json_decode($body, true, 512, JSON_THROW_ON_ERROR);
        } catch (\JsonException $e) {
            throw new GhanaGeoException('GhanaGeo returned invalid JSON.', 'INVALID_RESPONSE', $response->getStatusCode(), previous: $e);
        }
        if (! is_array($decoded)) {
            throw new GhanaGeoException('GhanaGeo returned a non-object response.', 'INVALID_RESPONSE', $response->getStatusCode());
        }

        return $decoded;
    }

    /** @param array<string, scalar|null> $query */
    private function send(string $path, string $accept = 'application/json', array $query = [], ?RequestOptions $options = null): ResponseInterface
    {
        $query = array_filter($query, static fn (mixed $v): bool => $v !== null);
        $uri = rtrim($this->baseUrl, '/').'/'.ltrim($path, '/').($query === [] ? '' : '?'.http_build_query($query, '', '&', PHP_QUERY_RFC3986));
        $attempt = 0;
        $started = hrtime(true);
        while (true) {
            $attempt++;
            $request = $this->requests->createRequest('GET', $uri)->withHeader('Accept', $accept)->withHeader('Accept-Encoding', 'identity')->withHeader('User-Agent', 'ghanageo-php/'.Version::SDK);
            foreach ($this->headers as $name => $value) {
                if (strcasecmp($name, 'Authorization') === 0) {
                    throw new \InvalidArgumentException('Use apiKey for authorization.');
                } $request = $request->withHeader($name, $value);
            }
            if ($this->apiKey !== null && $this->apiKey !== '') {
                $request = $request->withHeader('Authorization', 'Bearer '.$this->apiKey);
            }
            try {
                $timeout = $this->defaultTimeoutSeconds;
                if ($options !== null && $options->timeoutSeconds !== null) {
                    $timeout = $options->timeoutSeconds;
                }
                $response = $timeout !== null && $this->http instanceof DeadlineAwareClientInterface
                    ? $this->http->sendRequestWithTimeout($request, $timeout)
                    : $this->http->sendRequest($request);
            } catch (ClientExceptionInterface $e) {
                if ($this->retry !== null && $attempt < $this->retry->maxAttempts) {
                    $this->sleep($this->retry->delayFor($attempt, null));

                    continue;
                } throw new TransportException('GhanaGeo transport failed.', 'TRANSPORT_ERROR', previous: $e);
            }
            if ($this->retry !== null && $attempt < $this->retry->maxAttempts && in_array($response->getStatusCode(), $this->retry->statuses, true)) {
                $response->getBody()->close();
                $this->sleep($this->retry->delayFor($attempt, $response->getHeaderLine('Retry-After') ?: null));

                continue;
            }
            $this->emit(['operation' => 'GET', 'status' => $response->getStatusCode(), 'attempts' => $attempt, 'durationMs' => (hrtime(true) - $started) / 1_000_000]);
            if ($response->getStatusCode() < 200 || $response->getStatusCode() >= 300) {
                throw $this->apiError($response);
            }

            return $response;
        }
    }

    private function apiError(ResponseInterface $response): GhanaGeoException
    {
        $body = $this->boundedBody($response, min($this->maxBodyBytes, 1_048_576));
        $payload = json_decode($body, true);
        $error = is_array($payload) && isset($payload['error']) && is_array($payload['error']) ? $payload['error'] : [];

        return new GhanaGeoException((string) ($error['message'] ?? 'GhanaGeo request failed.'), (string) ($error['code'] ?? 'HTTP_ERROR'), $response->getStatusCode(), isset($error['requestId']) ? (string) $error['requestId'] : null, isset($error['docs']) ? (string) $error['docs'] : null, isset($error['details']) && is_array($error['details']) ? $error['details'] : []);
    }

    private function boundedBody(ResponseInterface $response, int $limit): string
    {
        $declared = $response->getHeaderLine('Content-Length');
        if ($declared !== '' && (int) $declared > $limit) {
            throw new GhanaGeoException('Response exceeds configured body limit.', 'PAYLOAD_TOO_LARGE', $response->getStatusCode());
        }
        $body = $response->getBody();
        $out = '';
        try {
            while (! $body->eof()) {
                $out .= $body->read(min(65_536, $limit - strlen($out) + 1));
                if (strlen($out) > $limit) {
                    throw new GhanaGeoException('Response exceeds configured body limit.', 'PAYLOAD_TOO_LARGE', $response->getStatusCode());
                }
            }
        } finally {
            $body->close();
        }

        return $out;
    }

    private function consumeDownload(ResponseInterface $response, ?string $destination, ?string $checksum): string
    {
        $temporary = null;
        $handle = null;
        $bytes = '';
        $size = 0;
        $hash = hash_init('sha256');
        if ($destination !== null) {
            $dir = dirname($destination);
            if (! is_dir($dir) || ($temporary = tempnam($dir, '.ghanageo-')) === false || ($handle = fopen($temporary, 'wb')) === false) {
                throw new DownloadException('Unable to create temporary download.', 'WRITE_FAILED');
            }
        }
        try {
            $body = $response->getBody();
            while (! $body->eof()) {
                $chunk = $body->read(min(65_536, $this->maxDownloadBytes - $size + 1));
                $size += strlen($chunk);
                if ($size > $this->maxDownloadBytes) {
                    throw new DownloadException('Download exceeds configured limit.', 'PAYLOAD_TOO_LARGE');
                }
                hash_update($hash, $chunk);
                if ($handle !== null) {
                    if (fwrite($handle, $chunk) !== strlen($chunk)) {
                        throw new DownloadException('Unable to write download.', 'WRITE_FAILED');
                    }
                } else {
                    $bytes .= $chunk;
                }
            }
            $actual = hash_final($hash);
            $expected = $checksum === null ? null : (str_starts_with($checksum, 'sha256:') ? substr($checksum, 7) : $checksum);
            if ($expected !== null && ! hash_equals(strtolower($expected), $actual)) {
                throw new DownloadException('Dataset checksum mismatch.', 'CHECKSUM_MISMATCH');
            }
            if ($handle !== null) {
                fclose($handle);
                $handle = null;
                if (! rename((string) $temporary, (string) $destination)) {
                    throw new DownloadException('Unable to atomically store download.', 'WRITE_FAILED');
                } $temporary = null;
            }

            return $destination === null ? $bytes : '';
        } finally {
            $response->getBody()->close();
            if (is_resource($handle)) {
                fclose($handle);
            }
            if ($temporary !== null && is_file($temporary)) {
                @unlink($temporary);
            }
        }
    }

    private function sleep(int $milliseconds): void
    {
        if ($milliseconds > 0) {
            usleep($milliseconds * 1_000);
        }
    }

    /** @param array<string, scalar|null> $event */
    private function emit(array $event): void
    {
        $this->logger->debug('GhanaGeo request completed.', $event);
        if ($this->telemetry !== null) {
            ($this->telemetry)($event);
        }
    }

    private static function segment(string $value): string
    {
        if ($value === '') {
            throw new \InvalidArgumentException('Path segment cannot be empty.');
        }

        return rawurlencode($value);
    }
}
