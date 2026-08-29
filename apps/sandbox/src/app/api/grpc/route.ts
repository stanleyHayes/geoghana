import * as grpc from "@grpc/grpc-js";
import * as protoLoader from "@grpc/proto-loader";
import path from "node:path";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

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
const attempts = new Map<string, { count: number; resetAt: number }>();

type GrpcClient = grpc.Client & Record<string, (...args: never[]) => unknown>;

function protoPath() {
  return path.resolve(process.cwd(), "../../proto/ghanageo/v1/geography.proto");
}

function client(): GrpcClient {
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
  return new Service(address, grpc.credentials.createInsecure()) as GrpcClient;
}

export async function POST(request: Request) {
  const clientId = request.headers.get("x-forwarded-for")?.split(",")[0]?.trim() || "local";
  const now = Date.now();
  const current = attempts.get(clientId);
  const allowance = !current || current.resetAt <= now ? { count: 0, resetAt: now + WINDOW_MS } : current;
  allowance.count += 1;
  attempts.set(clientId, allowance);
  if (allowance.count > REQUESTS_PER_WINDOW) {
    return Response.json(
      { error: "Anonymous gRPC sandbox limit reached. Try again shortly." },
      { status: 429, headers: { "Retry-After": String(Math.ceil((allowance.resetAt - now) / 1000)) } },
    );
  }

  if (Number(request.headers.get("content-length") ?? 0) > 16_384) {
    return Response.json({ error: "Request payload exceeds the 16 KiB sandbox limit." }, { status: 413 });
  }

  let input: { method?: string; payload?: unknown };
  try {
    input = await request.json();
  } catch {
    return Response.json({ error: "The request body must be valid JSON." }, { status: 400 });
  }

  if (!input.method || !ALLOWED_METHODS.has(input.method)) {
    return Response.json({ error: "That gRPC method is not available in the anonymous sandbox." }, { status: 400 });
  }

  const grpcClient = client();
  if (input.method === "StreamDatasetChanges") {
    const streamCall = grpcClient[input.method] as unknown as (payload: unknown) => grpc.ClientReadableStream<unknown>;
    const call = streamCall.call(grpcClient, input.payload ?? {});
    const encoder = new TextEncoder();
    let closed = false;
    const body = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(encoder.encode(`${JSON.stringify({ connected: true, message: "Waiting for dataset changes…" })}\n`));
        call.on("data", (change) => {
          if (!closed) controller.enqueue(encoder.encode(`${JSON.stringify(change)}\n`));
        });
        call.on("error", (error: grpc.ServiceError) => {
          if (closed) return;
          closed = true;
          controller.enqueue(encoder.encode(`${JSON.stringify({ error: error.details, code: error.code })}\n`));
          controller.close();
          grpcClient.close();
        });
        call.on("end", () => {
          if (closed) return;
          closed = true;
          controller.close();
          grpcClient.close();
        });
      },
      cancel() { closed = true; call.cancel(); grpcClient.close(); },
    });
    return new Response(body, { headers: { "Content-Type": "application/x-ndjson", "Cache-Control": "no-store" } });
  }
  try {
    const call = grpcClient[input.method] as unknown as (payload: unknown, callback: grpc.requestCallback<unknown>) => grpc.ClientUnaryCall;
    if (!call) throw new Error(`The ${input.method} gRPC method is unavailable.`);
    const result = await new Promise<unknown>((resolve, reject) => {
      call.call(grpcClient, input.payload ?? {}, (error, response) => {
        if (error) reject(error);
        else resolve(response);
      });
    });
    return Response.json(result, { headers: { "Cache-Control": "no-store" } });
  } catch (error) {
    const failure = error as grpc.ServiceError;
    return Response.json(
      { error: failure.details || "The gRPC request failed.", code: failure.code },
      { status: failure.code === grpc.status.INVALID_ARGUMENT ? 400 : 502 },
    );
  } finally {
    grpcClient.close();
  }
}
