<?php

declare(strict_types=1);
use GhanaGeo\Client;

return [
    'base_url' => getenv('GHANAGEO_BASE_URL') ?: Client::DEFAULT_BASE_URL,
    'api_key' => getenv('GHANAGEO_API_KEY') ?: null,
    'retry_attempts' => 3,
    'telemetry' => false,
    'max_body_bytes' => Client::DEFAULT_MAX_BODY_BYTES,
    'max_download_bytes' => Client::DEFAULT_MAX_DOWNLOAD_BYTES,
    'timeout_seconds' => 10.0,
];
