<?php

declare(strict_types=1);

namespace GhanaGeo\Transport;

use Psr\Http\Client\ClientInterface;
use Psr\Http\Message\RequestInterface;
use Psr\Http\Message\ResponseInterface;

interface DeadlineAwareClientInterface extends ClientInterface
{
    public function sendRequestWithTimeout(RequestInterface $request, float $timeoutSeconds): ResponseInterface;
}
