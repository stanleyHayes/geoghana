<?php

declare(strict_types=1);

use GhanaGeo\Client;
use GhanaGeo\Version;
use GuzzleHttp\Client as GuzzleClient;
use GuzzleHttp\Psr7\HttpFactory;

require __DIR__.'/vendor/autoload.php';

$factory = new HttpFactory;
$client = new Client(new GuzzleClient(['http_errors' => false]), $factory, 'https://example.invalid/v1', retry: null);
if (Version::API !== 'v1' || $client->testedDatasetVersion() !== '2026.08.3-ulid' || $client::DEFAULT_MAX_BODY_BYTES < 1) {
    throw new RuntimeException('Installed package smoke failed.');
}
