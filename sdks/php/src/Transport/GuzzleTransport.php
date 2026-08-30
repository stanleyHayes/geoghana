<?php

declare(strict_types=1);

namespace GhanaGeo\Transport;

use GuzzleHttp\ClientInterface as GuzzleClientInterface;
use Psr\Http\Message\RequestInterface;
use Psr\Http\Message\ResponseInterface;

final readonly class GuzzleTransport implements DeadlineAwareClientInterface
{
    public function __construct(private GuzzleClientInterface $client) {}

    public function sendRequest(RequestInterface $request): ResponseInterface
    {
        return $this->client->send($request, ['http_errors' => false]);
    }

    public function sendRequestWithTimeout(RequestInterface $request, float $timeoutSeconds): ResponseInterface
    {
        return $this->client->send($request, ['http_errors' => false, 'timeout' => $timeoutSeconds]);
    }
}
