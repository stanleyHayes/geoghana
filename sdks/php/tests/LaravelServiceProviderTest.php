<?php

declare(strict_types=1);

namespace GhanaGeo\Tests;

use GhanaGeo\Client;
use GhanaGeo\Laravel\GhanaGeoServiceProvider;
use GhanaGeo\Transport\GuzzleTransport;
use GuzzleHttp\Psr7\HttpFactory;
use Illuminate\Config\Repository;
use Illuminate\Container\Container;
use Illuminate\Contracts\Foundation\Application;
use Illuminate\Events\Dispatcher;
use PHPUnit\Framework\TestCase;
use Psr\Http\Client\ClientInterface;
use Psr\Http\Message\RequestFactoryInterface;

final class LaravelServiceProviderTest extends TestCase
{
    public function test_provider_registers_singleton(): void
    {
        $config = new Repository(['ghanageo' => ['base_url' => 'https://example.test/v1', 'retry_attempts' => 0]]);
        $factory = null;
        $app = $this->createMock(Application::class);
        $app->method('make')->willReturnCallback(fn (string $id): mixed => $id === 'config' ? $config : throw new \LogicException($id));
        $app->method('bound')->willReturn(false);
        $app->expects(self::exactly(3))->method('singleton')->willReturnCallback(function (string $id, callable $value) use (&$factory): void {
            if ($id === Client::class) {
                $factory = $value;
            }
        });
        $app->expects(self::once())->method('alias')->with(Client::class, 'ghanageo');
        (new GhanaGeoServiceProvider($app))->register();
        $container = new Container;
        $container->instance('config', $config);
        $container->instance('events', new Dispatcher($container));
        $httpFactory = new HttpFactory;
        $container->instance(ClientInterface::class, new GuzzleTransport(new \GuzzleHttp\Client));
        $container->instance(RequestFactoryInterface::class, $httpFactory);
        self::assertIsCallable($factory);
        self::assertInstanceOf(Client::class, $factory($container));
    }
}
