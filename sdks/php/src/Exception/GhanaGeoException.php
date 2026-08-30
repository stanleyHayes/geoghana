<?php

declare(strict_types=1);

namespace GhanaGeo\Exception;

class GhanaGeoException extends \RuntimeException
{
    /** @param array<string, mixed> $details */
    public function __construct(
        string $message,
        public readonly string $errorCode = 'INTERNAL',
        public readonly int $statusCode = 0,
        public readonly ?string $requestId = null,
        public readonly ?string $docs = null,
        public readonly array $details = [],
        ?\Throwable $previous = null,
    ) {
        parent::__construct($message, $statusCode, $previous);
    }
}
