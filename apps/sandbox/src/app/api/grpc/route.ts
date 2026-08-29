import * as grpc from "@grpc/grpc-js";
import * as protoLoader from "@grpc/proto-loader";
import path from "node:path";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

/**
 * The anonymous gRPC sandbox bridge.
 *
 * This endpoint lets an unauthenticated visitor drive the real gRPC service
 * from a browser, so every control here has to assume the caller is hostile
 * and controls every byte of the request — including its headers.
 */

const ALLOWED_METHODS = new Set([
  "ListRegions",
  "ListDistricts",
  "ListPlaces",
  "Search",
  "Autocomplete",
  "Geocode",
  "ReverseGeocode",
  "Nearby",
  "GetBoundary",
  "StreamDatasetChanges",
]);

const WINDOW_MS = 60_000;
const REQUESTS_PER_WINDOW = 30;
/**
 * A ceiling no header can lift.
 *
 * Per-identity limiting keyed on X-Forwarded-For is worth keeping as a
 * courtesy to honest callers sharing an office IP, but it is NOT a control:
 * the header is client-supplied, so anyone can mint a fresh budget by
 * incrementing it. This global window is what actually bounds the endpoint.
 */
const GLOBAL_REQUESTS_PER_WINDOW = 300;
/** Bounds the bookkeeping map so spoofed identities cannot exhaust memory. */
const MAX_TRACKED_IDENTITIES = 5_000;

const MAX_BODY_BYTES = 16_384;
/** A stream that is never closed by its client would otherwise live forever. */
const STREAM_TIMEOUT_MS = 30_000;
const UNARY_DEADLINE_MS = 10_000;
const MAX_CONCURRENT_STREAMS = 8;

type Window = { count: number; resetAt: number };
const attempts = new Map<string, Window>();
let globalWindow: Window = { count: 0, resetAt: 0 };
let openStreams = 0;

type GrpcClient = grpc.Client & Record<string, (...args: never[]) => unknown>;

function protoPath() {
  return path.resolve(process.cwd(), "../../proto/ghanageo/v1/geography.proto");
}

let shared: GrpcClient | undefined;

/**
 * One parsed contract, one channel, reused across requests.
 *
 * Parsing the proto per request cost ~44ms of SYNCHRONOUS work each time,
 * which on Node's single event loop serialises the whole route and stalls it
 * under load — a burst of requests failed upstream while the loop was blocked.
 * A gRPC channel is built to be long-lived and to multiplex concurrent calls,
 * so it is created once and never closed per request.
 */
function sharedClient(): GrpcClient {
  if (shared) return shared;
  const definition = protoLoader.loadSync(protoPath(), {
    defaults: true,
    enums: String,
    longs: String,
    oneofs: true,
  });
  const loaded = grpc.loadPackageDefinition(definition) as Record<string, Record<string, Record<string, grpc.ServiceClientConstructor>>>;
  const Service = loaded.ghanageo?.v1?.GeographyService;
  if (!Service) throw new Error("The GhanaGeo gRPC contract could not be loaded.");
  const address = process.env.GHANAGEO_GRPC_URL ?? "127.0.0.1:9190";
  shared = new Service(address, grpc.credentials.createInsecure()) as GrpcClient;
  return shared;
}

/** Drops windows that have expired, then bounds what is left. */
function sweep(now: number) {
  for (const [key, w] of attempts) {
    if (w.resetAt <= now) attempts.delete(key);
  }
  // Even with sweeping, a burst of spoofed identities inside one window could
  // grow this without limit. Past the cap, per-identity tracking is abandoned
  // and the global ceiling carries the load.
  if (attempts.size > MAX_TRACKED_IDENTITIES) attempts.clear();
}

function take(map: Map<string, Window>, key: string, now: number, limit: number) {
  const existing = map.get(key);
  const w = !existing || existing.resetAt <= now ? { count: 0, resetAt: now + WINDOW_MS } : existing;
  w.count += 1;
  map.set(key, w);
  return { exceeded: w.count > limit, retryAfter: Math.ceil((w.resetAt - now) / 1000) };
}

function tooMany(retryAfter: number, message: string) {
  return Response.json(
    { error: message },
    { status: 429, headers: { "Retry-After": String(Math.max(1, retryAfter)) } },
  );
}

/**
 * Reads the body while COUNTING the bytes that actually arrive.
 *
 * Content-Length is a claim, not a measurement: it is optional, and a chunked
 * request omits it entirely. Trusting it let a 200KB body through a 16KiB cap.
 * This reads the stream and aborts the moment the real total passes the limit,
 * so an oversized body is never fully buffered.
 */
async function readBounded(request: Request, limit: number): Promise<string | null> {
  if (!request.body) return "";
  const reader = request.body.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    total += value.byteLength;
    if (total > limit) {
      await reader.cancel().catch(() => {});
      return null;
    }
    chunks.push(value);
  }
  const joined = new Uint8Array(total);
  let offset = 0;
  for (const c of chunks) {
    joined.set(c, offset);
    offset += c.byteLength;
  }
  return new TextDecoder().decode(joined);
}

/**
 * Maps a gRPC status onto the HTTP status that means the same thing.
 *
 * Collapsing everything but INVALID_ARGUMENT into 502 was actively
 * misleading: the upstream's own rate limiter answers RESOURCE_EXHAUSTED,
 * and reporting that as "Bad Gateway" points whoever is debugging at a
 * broken service when the service is in fact working as designed.
 */
function httpStatusFor(code: grpc.status | undefined): number {
  switch (code) {
    case grpc.status.INVALID_ARGUMENT:
    case grpc.status.FAILED_PRECONDITION:
    case grpc.status.OUT_OF_RANGE:
      return 400;
    case grpc.status.UNAUTHENTICATED:
      return 401;
    case grpc.status.PERMISSION_DENIED:
      return 403;
    case grpc.status.NOT_FOUND:
      return 404;
    case grpc.status.ALREADY_EXISTS:
    case grpc.status.ABORTED:
      return 409;
    case grpc.status.RESOURCE_EXHAUSTED:
      return 429;
    case grpc.status.UNIMPLEMENTED:
      return 501;
    case grpc.status.UNAVAILABLE:
      return 503;
    case grpc.status.DEADLINE_EXCEEDED:
      return 504;
    default:
      return 502;
  }
}

export async function POST(request: Request) {
  const now = Date.now();
  sweep(now);

  // The global ceiling is checked FIRST, because it is the only one an
  // attacker cannot influence.
  if (globalWindow.resetAt <= now) globalWindow = { count: 0, resetAt: now + WINDOW_MS };
  globalWindow.count += 1;
  if (globalWindow.count > GLOBAL_REQUESTS_PER_WINDOW) {
    return tooMany(
      Math.ceil((globalWindow.resetAt - now) / 1000),
      "The anonymous gRPC sandbox is busy. Try again shortly.",
    );
  }

  const clientId = request.headers.get("x-forwarded-for")?.split(",")[0]?.trim() || "local";
  const perClient = take(attempts, clientId, now, REQUESTS_PER_WINDOW);
  if (perClient.exceeded) {
    return tooMany(perClient.retryAfter, "Anonymous gRPC sandbox limit reached. Try again shortly.");
  }

  const raw = await readBounded(request, MAX_BODY_BYTES);
  if (raw === null) {
    return Response.json({ error: "Request payload exceeds the 16 KiB sandbox limit." }, { status: 413 });
  }

  let input: { method?: string; payload?: unknown };
  try {
    input = JSON.parse(raw || "{}");
  } catch {
    return Response.json({ error: "The request body must be valid JSON." }, { status: 400 });
  }

  if (!input.method || !ALLOWED_METHODS.has(input.method)) {
    return Response.json({ error: "That gRPC method is not available in the anonymous sandbox." }, { status: 400 });
  }

  if (input.method === "StreamDatasetChanges") {
    if (openStreams >= MAX_CONCURRENT_STREAMS) {
      return tooMany(5, "Too many open sandbox streams. Try again shortly.");
    }
    const grpcClient = sharedClient();
    const streamCall = grpcClient[input.method] as unknown as (payload: unknown) => grpc.ClientReadableStream<unknown>;
    const call = streamCall.call(grpcClient, input.payload ?? {});
    const encoder = new TextEncoder();
    let closed = false;
    openStreams += 1;

    const body = new ReadableStream<Uint8Array>({
      start(controller) {
        // One place that releases every resource, however the stream ends.
        // Without a timer, a client that simply stops reading holds a gRPC
        // connection and a slot open indefinitely.
        const finish = (payload?: unknown) => {
          if (closed) return;
          closed = true;
          clearTimeout(timer);
          openStreams -= 1;
          if (payload !== undefined) {
            controller.enqueue(encoder.encode(`${JSON.stringify(payload)}\n`));
          }
          controller.close();
          call.cancel();
        };
        const timer = setTimeout(
          () => finish({ closed: true, reason: "Sandbox streams end after 30 seconds." }),
          STREAM_TIMEOUT_MS,
        );

        controller.enqueue(encoder.encode(`${JSON.stringify({ connected: true, message: "Waiting for dataset changes…" })}\n`));
        call.on("data", (change) => {
          if (!closed) controller.enqueue(encoder.encode(`${JSON.stringify(change)}\n`));
        });
        call.on("error", (error: grpc.ServiceError) => finish({ error: error.details, code: error.code }));
        call.on("end", () => finish());
      },
      cancel() {
        if (closed) return;
        closed = true;
        openStreams -= 1;
        call.cancel();
      },
    });
    return new Response(body, { headers: { "Content-Type": "application/x-ndjson", "Cache-Control": "no-store" } });
  }

  const grpcClient = sharedClient();
  try {
    const call = grpcClient[input.method] as unknown as (
      payload: unknown,
      options: grpc.CallOptions,
      callback: grpc.requestCallback<unknown>,
    ) => grpc.ClientUnaryCall;
    if (!call) throw new Error(`The ${input.method} gRPC method is unavailable.`);
    const result = await new Promise<unknown>((resolve, reject) => {
      // A deadline, so a slow upstream cannot pin a request handler open.
      call.call(grpcClient, input.payload ?? {}, { deadline: Date.now() + UNARY_DEADLINE_MS }, (error, response) => {
        if (error) reject(error);
        else resolve(response);
      });
    });
    return Response.json(result, { headers: { "Cache-Control": "no-store" } });
  } catch (error) {
    const failure = error as grpc.ServiceError;
    return Response.json(
      { error: failure.details || "The gRPC request failed.", code: failure.code },
      { status: httpStatusFor(failure.code) },
    );
  }
}
