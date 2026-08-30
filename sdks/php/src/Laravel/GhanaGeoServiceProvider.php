<?php

declare(strict_types=1);

namespace GhanaGeo\Laravel;

use GhanaGeo\Client;
use GhanaGeo\RetryPolicy;
use GhanaGeo\Transport\GuzzleTransport;
use GuzzleHttp\Client as GuzzleClient;
use GuzzleHttp\Psr7\HttpFactory;
use Illuminate\Container\Container;
use Illuminate\Support\ServiceProvider;
use Psr\Http\Client\ClientInterface;
use Psr\Http\Message\RequestFactoryInterface;

final class GhanaGeoServiceProvider extends ServiceProvider
{
    public function register(): void
    {
        $this->mergeConfigFrom(__DIR__.'/../../config/ghanageo.php', 'ghanageo');
        if (! $this->app->bound(ClientInterface::class) && class_exists(GuzzleClient::class)) {
            $this->app->singleton(ClientInterface::class, fn (): GuzzleTransport => new GuzzleTransport(new GuzzleClient));
        }
        if (! $this->app->bound(RequestFactoryInterface::class) && class_exists(HttpFactory::class)) {
            $this->app->singleton(RequestFactoryInterface::class, fn (): HttpFactory => new HttpFactory);
        }
        $this->app->singleton(Client::class, function (Container $app): Client {
            if (! $app->bound(ClientInterface::class) || ! $app->bound(RequestFactoryInterface::class)) {
                throw new \LogicException('GhanaGeo requires bound PSR-18 and PSR-17 implementations. Install guzzlehttp/guzzle or bind the interfaces.');
            }
            $attempts = (int) $app['config']->get('ghanageo.retry_attempts', 3);

            return new Client(
                http: $app->make(ClientInterface::class),
                requests: $app->make(RequestFactoryInterface::class),
                baseUrl: (string) $app['config']->get('ghanageo.base_url', Client::DEFAULT_BASE_URL),
                apiKey: $app['config']->get('ghanageo.api_key'),
                retry: $attempts > 0 ? new RetryPolicy($attempts) : null,
                logger: $app->bound('log') ? $app['log'] : null,
                telemetry: null,
                maxBodyBytes: (int) $app['config']->get('ghanageo.max_body_bytes', Client::DEFAULT_MAX_BODY_BYTES),
                maxDownloadBytes: (int) $app['config']->get('ghanageo.max_download_bytes', Client::DEFAULT_MAX_DOWNLOAD_BYTES),
                defaultTimeoutSeconds: (float) $app['config']->get('ghanageo.timeout_seconds', 10.0),
            );
        });
        $this->app->alias(Client::class, 'ghanageo');
    }

    public function boot(): void
    {
        $this->publishes([__DIR__.'/../../config/ghanageo.php' => $this->app->configPath('ghanageo.php')], 'ghanageo-config');
    }
}
