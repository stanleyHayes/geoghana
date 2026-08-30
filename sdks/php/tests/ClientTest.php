<?php

declare(strict_types=1);

namespace GhanaGeo\Tests;

use GhanaGeo\Client;
use GhanaGeo\Exception\GhanaGeoException;
use GhanaGeo\RequestOptions;
use GhanaGeo\RetryPolicy;
use GhanaGeo\Transport\DeadlineAwareClientInterface;
use GuzzleHttp\Psr7\HttpFactory;
use GuzzleHttp\Psr7\Response;
use PHPUnit\Framework\TestCase;
use Psr\Http\Message\RequestInterface;
use Psr\Http\Message\ResponseInterface;

final class ClientTest extends TestCase
{
    public function test_anonymous_region_mapping_and_escaping(): void
    {
        $http = new FakePsrClient([new Response(200, [], json_encode(['id' => 'r1', 'countryCode' => 'GH', 'name' => 'Ahafo', 'status' => 'ACTIVE', 'verificationStatus' => 'REFERENCE', 'provenance' => ['sourceId' => 'seed'], 'datasetVersion' => '2026.08'], JSON_THROW_ON_ERROR))]);
        $client = $this->client($http);
        $region = $client->region('r/1');
        self::assertSame('Ahafo', $region->name);
        self::assertSame('/v1/regions/r%2F1', $http->requests[0]->getUri()->getPath());
        self::assertFalse($http->requests[0]->hasHeader('Authorization'));
    }

    public function test_typed_error_preserves_contract_fields(): void
    {
        $http = new FakePsrClient([new Response(404, [], json_encode(['error' => ['code' => 'NOT_FOUND', 'message' => 'missing', 'requestId' => 'req-1', 'docs' => '/errors/not-found', 'details' => ['id' => 'x']]], JSON_THROW_ON_ERROR))]);
        try {
            $this->client($http)->place('x');
            self::fail('Expected exception.');
        } catch (GhanaGeoException $e) {
            self::assertSame('NOT_FOUND', $e->errorCode);
            self::assertSame(404, $e->statusCode);
            self::assertSame('req-1', $e->requestId);
            self::assertSame('x', $e->details['id']);
        }
    }

    public function test_paginator_stops_and_detects_repeated_cursor(): void
    {
        $payload = fn (string $cursor) => new Response(200, [], json_encode(['data' => [], 'datasetVersion' => 'v', 'nextCursor' => $cursor], JSON_THROW_ON_ERROR));
        $client = $this->client(new FakePsrClient([$payload('same'), $payload('same')]));
        $this->expectException(GhanaGeoException::class);
        iterator_to_array($client->pages(fn (?string $cursor) => $client->regions($cursor)));
    }

    public function test_retries_bounded_safe_get(): void
    {
        $http = new FakePsrClient([new Response(503), new Response(200, [], json_encode(['data' => [], 'datasetVersion' => 'v'], JSON_THROW_ON_ERROR))]);
        $client = $this->client($http, new RetryPolicy(2, 0, 0));
        self::assertSame('v', $client->regions()->datasetVersion);
        self::assertCount(2, $http->requests);
    }

    public function test_download_checksum_and_atomic_destination(): void
    {
        $bytes = 'dataset';
        $path = sys_get_temp_dir().'/ghanageo-test-'.bin2hex(random_bytes(4));
        $client = $this->client(new FakePsrClient([new Response(200, [], $bytes)]));
        self::assertSame('', $client->downloadDatasetArtifact('2026.08.1', 'regions', 'json', $path, hash('sha256', $bytes)));
        self::assertSame($bytes, file_get_contents($path));
        unlink($path);
    }

    public function test_versions_and_retry_bounds(): void
    {
        $client = $this->client(new FakePsrClient([]));
        self::assertSame('v1', $client->apiVersion());
        self::assertNotSame('', $client->sdkVersion());
        self::assertSame('2026.08.3-ulid', $client->testedDatasetVersion());
        $this->expectException(\InvalidArgumentException::class);
        new RetryPolicy(7);
    }

    public function test_explicit_deadline_uses_capable_transport(): void
    {
        $factory = new HttpFactory;
        $transport = new class implements DeadlineAwareClientInterface
        {
            public ?float $timeout = null;

            public function sendRequest(RequestInterface $request): ResponseInterface
            {
                throw new \LogicException('deadline path required');
            }

            public function sendRequestWithTimeout(RequestInterface $request, float $timeoutSeconds): ResponseInterface
            {
                $this->timeout = $timeoutSeconds;

                return new Response(200, [], json_encode(['data' => [], 'datasetVersion' => 'v'], JSON_THROW_ON_ERROR));
            }
        };
        $client = new Client($transport, $factory, 'https://example.test/v1', retry: null);
        $client->search('Kumasi', options: new RequestOptions(0.25));
        self::assertSame(0.25, $transport->timeout);
    }

    private function client(FakePsrClient $http, ?RetryPolicy $retry = null): Client
    {
        $factory = new HttpFactory;

        return new Client($http, $factory, 'https://example.test/v1', retry: $retry);
    }
}
