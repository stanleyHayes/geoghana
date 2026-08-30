<?php

declare(strict_types=1);

namespace GhanaGeo\Laravel\Facades;

use Illuminate\Support\Facades\Facade;

final class GhanaGeo extends Facade
{
    protected static function getFacadeAccessor(): string
    {
        return 'ghanageo';
    }
}
