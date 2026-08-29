export type GhanaGeoClientOptions = {
  baseUrl?: string;
  apiKey?: string;
  fetcher?: typeof fetch;
};

export class GhanaGeoError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly code?: string,
    public readonly requestId?: string,
  ) {
    super(message);
    this.name = "GhanaGeoError";
  }
}

export class GhanaGeoClient {
  readonly baseUrl: string;
  private readonly apiKey?: string;
  private readonly fetcher: typeof fetch;

  constructor(options: GhanaGeoClientOptions = {}) {
    this.baseUrl = (options.baseUrl ?? "https://api.geo.digitalghana.dev/v1").replace(/\/$/, "");
    this.apiKey = options.apiKey;
    this.fetcher = options.fetcher ?? fetch;
  }

  async get<T>(path: string, params?: Record<string, string | number | boolean | undefined | null>, signal?: AbortSignal): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`);
    for (const [key, value] of Object.entries(params ?? {})) {
      if (value !== undefined && value !== null && value !== "") url.searchParams.set(key, String(value));
    }
    const headers: Record<string, string> = { Accept: "application/json" };
    if (this.apiKey) headers.Authorization = `Bearer ${this.apiKey}`;
    const res = await this.fetcher(url, { headers, signal });
    const body = await res.json().catch(() => ({}));
    if (!res.ok) {
      const err = body?.error ?? {};
      throw new GhanaGeoError(err.message ?? `GhanaGeo request failed (${res.status})`, res.status, err.code, err.requestId);
    }
    return body as T;
  }

  async graphql<TData, TVariables extends Record<string, unknown> = Record<string, unknown>>(
    query: string,
    variables?: TVariables,
    signal?: AbortSignal,
  ): Promise<TData> {
    const endpoint = this.baseUrl.replace(/\/v1$/, "") + "/graphql";
    const headers: Record<string, string> = { "Content-Type": "application/json", Accept: "application/json" };
    if (this.apiKey) headers.Authorization = `Bearer ${this.apiKey}`;
    const res = await this.fetcher(endpoint, {
      method: "POST",
      headers,
      body: JSON.stringify({ query, variables }),
      signal,
    });
    const body = await res.json();
    if (!res.ok || body.errors?.length) {
      const first = body.errors?.[0];
      throw new GhanaGeoError(first?.message ?? `GraphQL request failed (${res.status})`, res.status, first?.extensions?.code, first?.extensions?.requestId);
    }
    return body.data as TData;
  }
}
