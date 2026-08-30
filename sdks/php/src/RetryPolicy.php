<?php

declare(strict_types=1);

namespace GhanaGeo;

final readonly class RetryPolicy
{
    /** @param list<int> $statuses */
    public function __construct(
        public int $maxAttempts = 3,
        public int $baseDelayMilliseconds = 100,
        public int $maxDelayMilliseconds = 2_000,
        public array $statuses = [429, 502, 503, 504],
    ) {
        if ($maxAttempts < 1 || $maxAttempts > 6 || $baseDelayMilliseconds < 0 || $maxDelayMilliseconds < 0 || $maxDelayMilliseconds < $baseDelayMilliseconds) {
            throw new \InvalidArgumentException('Invalid retry policy.');
        }
    }

    public function delayFor(int $attempt, ?string $retryAfter): int
    {
        if ($attempt < 1 || $attempt >= $this->maxAttempts) {
            throw new \InvalidArgumentException('Retry attempt is outside the configured policy.');
        }
        if ($retryAfter !== null && ctype_digit($retryAfter)) {
            if (strlen($retryAfter) > 6) {
                return $this->maxDelayMilliseconds;
            }

            return min($this->maxDelayMilliseconds, (int) $retryAfter * 1_000);
        }

        $delay = min($this->maxDelayMilliseconds, $this->baseDelayMilliseconds * (2 ** max(0, $attempt - 1)));
        if ($delay === 0) {
            return 0;
        }

        return random_int((int) floor($delay / 2), max(1, $delay));
    }
}
