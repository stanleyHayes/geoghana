export const examples = [
  {
    id: "typescript", label: "TypeScript", source: "scripts/docs-sdk/examples/typescript.ts",
    install: "pnpm add @ghanageo/client", runtime: "Node.js 22+ / modern browsers",
    protocols: ["REST", "GraphQL"], cancellation: "AbortSignal", retry: "Two safe-read retries; max five",
    pagination: "Async page and item iterators", errors: "GhanaGeoError",
  },
  {
    id: "react", label: "React", source: "packages/react/examples/next16/app/page.tsx",
    install: "pnpm add @ghanageo/react @tanstack/react-query", runtime: "React 18.3+ / 19",
    protocols: ["REST"], cancellation: "TanStack Query AbortSignal", retry: "Client retry policy",
    pagination: "Infinite-query page helpers", errors: "GhanaGeoError",
  },
  {
    id: "python", label: "Python", source: "sdks/python/examples/async_search.py",
    install: "pip install ghanageo", runtime: "Python 3.10+",
    protocols: ["REST"], cancellation: "asyncio task cancellation", retry: "RetryConfig; max five",
    pagination: "Async page iterators", errors: "GhanaGeoError",
  },
  {
    id: "go", label: "Go", source: "sdks/go/example_test.go",
    install: "go get github.com/ghanageo/ghanageo-go", runtime: "Go 1.23+",
    protocols: ["REST"], cancellation: "context.Context", retry: "Functional retry option; max five",
    pagination: "Next(ctx), Page(), Err()", errors: "*ghanageo.Error",
  },
  {
    id: "dart", label: "Dart", source: "sdks/dart/example/regions.dart",
    install: "dart pub add ghanageo", runtime: "Dart 3.5+",
    protocols: ["REST"], cancellation: "CancellationToken", retry: "RetryPolicy; max five",
    pagination: "Stream<Page<T>>", errors: "GhanaGeoException",
  },
  {
    id: "java", label: "Java", source: "sdks/java/examples/src/main/java/dev/ghanageo/examples/SearchExample.java",
    install: "implementation(\"dev.ghanageo:ghanageo-java:2.0.0\")", runtime: "Java 21+",
    protocols: ["REST"], cancellation: "CancellationToken / CompletableFuture", retry: "Builder retry policy; max five",
    pagination: "Flow.Publisher<Page<T>>", errors: "GhanaGeoException",
  },
  {
    id: "dotnet", label: "C# / .NET", source: "sdks/dotnet/examples/ConsoleExample/Program.cs",
    install: "dotnet add package GhanaGeo", runtime: ".NET 8+",
    protocols: ["REST"], cancellation: "CancellationToken", retry: "Options retry policy; max five",
    pagination: "IAsyncEnumerable<Page<T>>", errors: "GhanaGeoException",
  },
  {
    id: "php", label: "PHP", source: "sdks/php/examples/regions.php",
    install: "composer require ghanageo/ghanageo-php", runtime: "PHP 8.2+",
    protocols: ["REST"], cancellation: "RequestOptions deadline/token", retry: "RetryPolicy; max five",
    pagination: "Generator<Page>", errors: "GhanaGeoException",
  },
  {
    id: "curl", label: "curl", source: "scripts/docs-sdk/examples/curl.sh",
    install: "No install beyond curl", runtime: "curl 8+",
    protocols: ["REST"], cancellation: "Shell process signal / --max-time", retry: "Explicit --retry for safe reads",
    pagination: "Follow nextCursor explicitly", errors: "HTTP status plus JSON error envelope",
  },
];
