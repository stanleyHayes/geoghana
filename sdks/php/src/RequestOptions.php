<?php

declare(strict_types=1);

namespace GhanaGeo;

final readonly class RequestOptions
{
    public function __construct(public ?float $timeoutSeconds = null)
    {
        if ($timeoutSeconds !== null && (! is_finite($timeoutSeconds) || $timeoutSeconds <= 0)) {
            throw new \InvalidArgumentException('Request timeout must be a finite positive number.');
        }
    }
}
