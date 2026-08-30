<?php

declare(strict_types=1);

use GhanaGeo\Client;
use GhanaGeo\Exception\GhanaGeoException;
use GhanaGeo\Exception\TransportException;
use GhanaGeo\RequestOptions;
use GhanaGeo\RetryPolicy;
use GhanaGeo\Transport\DeadlineAwareClientInterface;
use GhanaGeo\Transport\GuzzleTransport;
use GhanaGeo\Version;
use GuzzleHttp\Client as GuzzleClient;
use GuzzleHttp\Psr7\HttpFactory;
use GuzzleHttp\Psr7\Response;
use Psr\Http\Message\RequestInterface;
use Psr\Http\Message\ResponseInterface;

require dirname(__DIR__).'/vendor/autoload.php';

$arguments = $_SERVER['argv'] ?? [];
if (count($arguments) !== 4) {
    fwrite(STDERR, "usage: php conformance/runner.php cases.json base-url report.json\n");
    exit(2);
}
$export = json_decode((string) file_get_contents($arguments[1]), true, 512, JSON_THROW_ON_ERROR);
$baseUrl = rtrim($arguments[2], '/');
$results = [];
$evidence = [];
$versions = [];

final class RecordingTransport implements DeadlineAwareClientInterface
{
    /** @var array<string,mixed>|null */
    public ?array $requestEvidence = null;

    /** @var array<string,mixed>|null */
    public ?array $responseEvidence = null;

    public function __construct(private readonly DeadlineAwareClientInterface $delegate) {}

    public function sendRequest(RequestInterface $request): ResponseInterface
    {
        return $this->capture($request, null);
    }

    public function sendRequestWithTimeout(RequestInterface $request, float $timeoutSeconds): ResponseInterface
    {
        return $this->capture($request, $timeoutSeconds);
    }

    private function capture(RequestInterface $request, ?float $timeout): ResponseInterface
    {
        $headers = array_values(array_filter(array_map('strtolower', array_keys($request->getHeaders())), fn (string $name): bool => ! in_array($name, ['authorization', 'cookie'], true)));
        sort($headers);
        $this->requestEvidence = ['method' => $request->getMethod(), 'path' => $request->getUri()->getPath(), 'query' => $request->getUri()->getQuery(), 'headerNames' => $headers, 'bodyDigest' => digest((string) $request->getBody())];
        $response = $timeout === null ? $this->delegate->sendRequest($request) : $this->delegate->sendRequestWithTimeout($request, $timeout);
        $body = (string) $response->getBody();
        $this->responseEvidence = ['status' => $response->getStatusCode(), 'contentType' => $response->getHeaderLine('Content-Type'), 'bodyDigest' => digest($body)];

        return new Response($response->getStatusCode(), $response->getHeaders(), $body, $response->getProtocolVersion(), $response->getReasonPhrase());
    }
}

foreach ($export['cases'] as $case) {
    if (! in_array('rest', $case['protocols'], true)) {
        continue;
    }
    $factory = new HttpFactory;
    $input = $case['input'] ?? [];
    $auth = $input['auth'] ?? null;
    $apiKey = is_string($auth) && $auth !== 'omitted' ? $auth : null;
    $transport = new RecordingTransport(new GuzzleTransport(new GuzzleClient(['http_errors' => false])));
    $client = new Client($transport, $factory, $baseUrl, $apiKey, new RetryPolicy(1), headers: ['X-Conformance-Case' => $case['id']]);
    $status = 'passed';
    $message = null;
    $value = null;
    try {
        $value = invoke($client, $case);
        if (($case['expect']['outcome'] ?? 'success') === 'error') {
            $status = 'failed';
            $message = 'expected typed error, received success';
        } else {
            validateShape(normalize($value), $case['expect']['shape'] ?? []);
        }
    } catch (GhanaGeoException $error) {
        if ($case['id'] === 'semantic.cancellation-propagates' && $error instanceof TransportException) {
            $value = ['cancelled' => true];
        } elseif (($case['expect']['outcome'] ?? '') !== 'error') {
            $status = 'failed';
            $message = $error->errorCode.': '.$error->getMessage();
        } elseif ($error->errorCode !== ($case['expect']['error']['code'] ?? null) || $error->statusCode !== ($case['expect']['httpStatus'] ?? null)) {
            $status = 'failed';
            $message = "error was {$error->errorCode}/{$error->statusCode}";
        } else {
            $value = ['error' => ['code' => $error->errorCode, 'message' => $error->getMessage(), 'requestId' => $error->requestId, 'docs' => $error->docs, 'details' => $error->details]];
            try {
                validateRequired($value['error'], $case['expect']['error']['requiredFields'] ?? []);
            } catch (Throwable $validation) {
                $status = 'failed';
                $message = $validation::class.': '.$validation->getMessage();
            }
        }
    } catch (Throwable $error) {
        $status = 'failed';
        $message = $error::class.': '.$error->getMessage();
    }
    $caseVersions = [];
    collectVersions(normalize($value), $caseVersions);
    foreach (array_keys($caseVersions) as $observedVersion) {
        $versions[$observedVersion] = true;
        if ($observedVersion !== $client->testedDatasetVersion()) {
            $status = 'failed';
            $message = "observed dataset {$observedVersion}, SDK tested metadata is {$client->testedDatasetVersion()}";
        }
    }
    $results[] = array_filter(['caseId' => $case['id'], 'status' => $status, 'protocols' => ['rest'], 'message' => $message], fn ($v) => $v !== null);
    $evidence[] = ['caseId' => $case['id'], 'protocol' => 'rest', 'requestDigest' => digest($transport->requestEvidence), 'responseDigest' => digest($transport->responseEvidence)];
}
$summary = ['passed' => count(array_filter($results, fn ($r) => $r['status'] === 'passed')), 'failed' => count(array_filter($results, fn ($r) => $r['status'] === 'failed')), 'skipped' => 0];
$report = ['schemaVersion' => 2, 'contract' => ['digest' => $export['contractDigest'], 'runnerVersion' => '1.0.0'], 'sdk' => ['language' => 'PHP', 'name' => 'ghanageo/ghanageo-php', 'version' => Version::SDK, 'supportedProtocols' => ['rest']], 'apiVersion' => Version::API, 'datasetVersion' => array_key_first($versions) ?? Version::TESTED_DATASET, 'summary' => $summary, 'evidenceDigest' => digest($evidence), 'evidence' => $evidence, 'results' => $results];
file_put_contents($arguments[3], json_encode($report, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES | JSON_THROW_ON_ERROR)."\n");
exit($summary['failed'] === 0 ? 0 : 1);

/** @param array<string,mixed> $case */
function invoke(Client $c, array $case): mixed
{
    $p = $case['input']['path'] ?? [];
    $q = $case['input']['query'] ?? [];
    $s = fn (string $k): ?string => isset($q[$k]) ? (string) $q[$k] : null;
    $i = fn (string $k): ?int => isset($q[$k]) && is_numeric($q[$k]) ? (int) $q[$k] : null;
    if ($case['id'] === 'error.invalid-argument') {
        return $c->raw('/places', ['limit' => 'invalid']);
    }

    return match ($case['operation']) {
        'listRegions' => $c->regions($s('cursor'), $i('limit')),
        'getRegion' => $c->region((string) $p['id']),
        'listRegionDistricts' => $c->regionDistricts((string) $p['id'], $s('cursor'), $i('limit')),
        'listDistricts' => $c->districts($s('regionId'), $s('q'), $s('cursor'), $i('limit')),
        'getDistrict' => $c->district((string) $p['id']),
        'listDistrictPlaces' => $c->districtPlaces((string) $p['id'], $s('cursor'), $i('limit'), $s('type')),
        'listPlaces' => $c->places($s('districtId'), $s('regionId'), $s('type'), $s('q'), $s('cursor'), $i('limit')),
        'getPlace' => $c->place((string) $p['id']),
        'search' => $c->search((string) ($q['q'] ?? 'Kumasi'), $s('regionId'), $s('districtId'), $s('type'), $i('limit'), $case['id'] === 'semantic.cancellation-propagates' ? new RequestOptions(0.01) : null),
        'autocomplete' => $c->autocomplete((string) $q['q'], $i('limit')),
        'geocode' => $c->geocode((string) $q['q'], $i('limit')),
        'reverseGeocode' => $c->reverseGeocode((float) ($q['lat'] ?? 6.6885), (float) ($q['lng'] ?? -1.6244)),
        'nearby' => $c->nearby((float) ($q['lat'] ?? 6.6885), (float) ($q['lng'] ?? -1.6244), (int) ($q['radius'] ?? 5000), (int) ($q['limit'] ?? 20)),
        'getBoundary' => $c->boundary((string) $p['id']),
        'listDatasets' => $c->datasets(),
        'listDatasetDownloads' => $c->datasetDownloads((string) $p['version']),
        'downloadDatasetArtifact' => $c->downloadDatasetArtifact((string) $p['version'], (string) $p['entity'], (string) $p['format']),
        'listRoads' => $c->roads(),
        'listPointsOfInterest' => $c->pointsOfInterest(),
        default => throw new LogicException('No PHP REST mapping for '.$case['operation']),
    };
}
function normalize(mixed $v): mixed
{
    return json_decode(json_encode($v, JSON_THROW_ON_ERROR), true, 512, JSON_THROW_ON_ERROR);
}
/** @param array<string,mixed> $shape */
function validateShape(mixed $value, array $shape): void
{
    if (($shape['type'] ?? '') === 'binary') {
        if (! is_string($value)) {
            throw new UnexpectedValueException('expected binary string');
        }

        return;
    } if (! is_array($value)) {
        throw new UnexpectedValueException('expected object');
    } validateRequired($value, $shape['required'] ?? []);
}
/**
 * @param  array<string,mixed>  $value
 * @param  array<int, string>  $paths
 */
function validateRequired(array $value, array $paths): void
{
    foreach ($paths as $path) {
        $node = $value;
        foreach (explode('.', $path) as $part) {
            if (! is_array($node) || ! array_key_exists($part, $node) || $node[$part] === null) {
                throw new UnexpectedValueException('missing '.$path);
            } $node = $node[$part];
        }
    }
}
function digest(mixed $v): string
{
    return hash('sha256', json_encode($v, JSON_UNESCAPED_SLASHES | JSON_THROW_ON_ERROR));
}
/** @param array<string,bool> $versions */
function collectVersions(mixed $v, array &$versions): void
{
    if (! is_array($v)) {
        return;
    } foreach ($v as $k => $child) {
        if ($k === 'datasetVersion' && is_string($child) && $child !== '') {
            $versions[$child] = true;
        } collectVersions($child, $versions);
    }
}
