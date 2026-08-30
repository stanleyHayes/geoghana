<?php

declare(strict_types=1);

use GhanaGeo\Client;
use GuzzleHttp\Client as GuzzleClient;
use GuzzleHttp\Psr7\HttpFactory;

require dirname(__DIR__).'/vendor/autoload.php';

$factory = new HttpFactory;
$client = new Client(new GuzzleClient(['http_errors' => false, 'timeout' => 10]), $factory);

foreach ($client->allRegions() as $region) {
    echo $region->name.PHP_EOL;
}
