package dev.ghanageo;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.JavaType;
import java.io.IOException;
import java.net.URI;
import java.net.URLEncoder;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.nio.ByteBuffer;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.MessageDigest;
import java.time.Duration;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.concurrent.CancellationException;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.CompletionException;
import java.util.concurrent.Flow;
import java.util.concurrent.CompletionStage;
import java.util.concurrent.SubmissionPublisher;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledFuture;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.ThreadLocalRandom;
import java.util.function.Consumer;
import java.util.function.Function;

/** Thread-safe synchronous and asynchronous GhanaGeo REST client. */
public final class GhanaGeoClient implements AutoCloseable {
  public static final String DEFAULT_BASE_URL = "https://api.geo.digitalghana.dev/v1";
  private static final Set<Integer> RETRYABLE = Set.of(429, 502, 503, 504);
  private final URI baseUri; private final String apiKey; private final HttpClient http;
  private final ObjectMapper json; private final int maxRetries; private final Duration retryDelay;
  private final int maxJsonBytes; private final long maxArtifactBytes; private final Consumer<Models.TelemetryEvent> telemetry; private final Map<String,String> defaultHeaders;
  private final Consumer<Models.WireEvidence> wireObserver;
  private final ScheduledExecutorService scheduler=Executors.newSingleThreadScheduledExecutor(r->{Thread t=new Thread(r,"ghanageo-retry");t.setDaemon(true);return t;});

  private GhanaGeoClient(Builder b) {
    baseUri = URI.create(stripSlash(b.baseUrl)); apiKey = b.apiKey; http = b.httpClient;
    json = b.objectMapper; maxRetries = Math.min(5, Math.max(0, b.maxRetries)); retryDelay = b.retryDelay;
    maxJsonBytes = Math.min(64 * 1024 * 1024, Math.max(1, b.maxJsonBytes)); defaultHeaders=Map.copyOf(b.defaultHeaders);
    maxArtifactBytes = Math.min(1024L * 1024 * 1024, Math.max(1, b.maxArtifactBytes)); telemetry = b.telemetry;wireObserver=b.wireObserver;
  }
  public static Builder builder() { return new Builder(); }
  public static GhanaGeoClient create() { return builder().build(); }
  public URI baseUri() { return baseUri; }
  public String apiVersion(){return Version.API;} public String sdkVersion(){return Version.SDK;} public String testedDatasetVersion(){return Version.TESTED_DATASET;}

  public Models.RegionPage regions(Map<String,?> q,CancellationToken t){return read("/regions",q,Models.RegionPage.class,t);} public Models.RegionPage regions(Map<String,?>q){return regions(q,CancellationToken.none());}
  public Models.Region region(String id,CancellationToken t){return read("/regions/"+segment(id),Map.of(),Models.Region.class,t);} public Models.Region region(String id){return region(id,CancellationToken.none());}
  public Models.DistrictPage regionDistricts(String id,Map<String,?>q,CancellationToken t){return read("/regions/"+segment(id)+"/districts",q,Models.DistrictPage.class,t);} public Models.DistrictPage regionDistricts(String id,Map<String,?>q){return regionDistricts(id,q,CancellationToken.none());}
  public Models.DistrictPage districts(Map<String,?>q,CancellationToken t){return read("/districts",q,Models.DistrictPage.class,t);} public Models.DistrictPage districts(Map<String,?>q){return districts(q,CancellationToken.none());}
  public Models.District district(String id,CancellationToken t){return read("/districts/"+segment(id),Map.of(),Models.District.class,t);} public Models.District district(String id){return district(id,CancellationToken.none());}
  public Models.PlacePage districtPlaces(String id,Map<String,?>q,CancellationToken t){return read("/districts/"+segment(id)+"/places",q,Models.PlacePage.class,t);} public Models.PlacePage districtPlaces(String id,Map<String,?>q){return districtPlaces(id,q,CancellationToken.none());}
  public Models.PlacePage places(Map<String,?>q,CancellationToken t){return read("/places",q,Models.PlacePage.class,t);} public Models.PlacePage places(Map<String,?>q){return places(q,CancellationToken.none());}
  public Models.Place place(String id,CancellationToken t){return read("/places/"+segment(id),Map.of(),Models.Place.class,t);} public Models.Place place(String id){return place(id,CancellationToken.none());}
  public Models.SearchPage search(String q,Map<String,?>p,CancellationToken t){return read("/search",with(p,"q",q),Models.SearchPage.class,t);} public Models.SearchPage search(String q,Map<String,?>p){return search(q,p,CancellationToken.none());}
  public Models.SearchPage autocomplete(String q,int l,CancellationToken t){return read("/autocomplete",Map.of("q",q,"limit",l),Models.SearchPage.class,t);} public Models.SearchPage autocomplete(String q,int l){return autocomplete(q,l,CancellationToken.none());}
  public Models.SearchPage geocode(String q,int l,CancellationToken t){return read("/geocode",Map.of("q",q,"limit",l),Models.SearchPage.class,t);} public Models.SearchPage geocode(String q,int l){return geocode(q,l,CancellationToken.none());}
  public Models.ReverseResult reverseGeocode(double lat,double lng,CancellationToken t){return read("/reverse",Map.of("lat",lat,"lng",lng),Models.ReverseResult.class,t);} public Models.ReverseResult reverseGeocode(double lat,double lng){return reverseGeocode(lat,lng,CancellationToken.none());}
  public Models.PlacePage nearby(double lat,double lng,int r,int l,CancellationToken t){return read("/nearby",Map.of("lat",lat,"lng",lng,"radius",r,"limit",l),Models.PlacePage.class,t);} public Models.PlacePage nearby(double lat,double lng,int r,int l){return nearby(lat,lng,r,l,CancellationToken.none());}
  public Models.BoundaryFeature boundary(String id,CancellationToken t){return read("/boundaries/"+segment(id),Map.of(),Models.BoundaryFeature.class,t);} public Models.BoundaryFeature boundary(String id){return boundary(id,CancellationToken.none());}
  public Models.DatasetPage datasets(CancellationToken t){return read("/datasets",Map.of(),Models.DatasetPage.class,t);} public Models.DatasetPage datasets(){return datasets(CancellationToken.none());}
  public Models.DownloadList datasetDownloads(String v,CancellationToken t){return read("/datasets/"+segment(v)+"/downloads",Map.of(),Models.DownloadList.class,t);} public Models.DownloadList datasetDownloads(String v){return datasetDownloads(v,CancellationToken.none());}
  public Models.OpenData roads(CancellationToken t){return openData("/roads",t);} public Models.OpenData roads(){return roads(CancellationToken.none());}
  public Models.OpenData pointsOfInterest(CancellationToken t){return openData("/pois",t);} public Models.OpenData pointsOfInterest(){return pointsOfInterest(CancellationToken.none());}

  public CompletableFuture<Models.RegionPage> regionsAsync(Map<String,?>q,CancellationToken t){return readAsync("/regions",q,Models.RegionPage.class,t);} public CompletableFuture<Models.RegionPage> regionsAsync(Map<String,?>q){return regionsAsync(q,CancellationToken.none());}
  public CompletableFuture<Models.Region> regionAsync(String id,CancellationToken t){return readAsync("/regions/"+segment(id),Map.of(),Models.Region.class,t);} public CompletableFuture<Models.Region> regionAsync(String id){return regionAsync(id,CancellationToken.none());}
  public CompletableFuture<Models.DistrictPage> regionDistrictsAsync(String id,Map<String,?>q,CancellationToken t){return readAsync("/regions/"+segment(id)+"/districts",q,Models.DistrictPage.class,t);} public CompletableFuture<Models.DistrictPage> regionDistrictsAsync(String id,Map<String,?>q){return regionDistrictsAsync(id,q,CancellationToken.none());}
  public CompletableFuture<Models.DistrictPage> districtsAsync(Map<String,?>q,CancellationToken t){return readAsync("/districts",q,Models.DistrictPage.class,t);} public CompletableFuture<Models.DistrictPage> districtsAsync(Map<String,?>q){return districtsAsync(q,CancellationToken.none());}
  public CompletableFuture<Models.District> districtAsync(String id,CancellationToken t){return readAsync("/districts/"+segment(id),Map.of(),Models.District.class,t);} public CompletableFuture<Models.District> districtAsync(String id){return districtAsync(id,CancellationToken.none());}
  public CompletableFuture<Models.PlacePage> districtPlacesAsync(String id,Map<String,?>q,CancellationToken t){return readAsync("/districts/"+segment(id)+"/places",q,Models.PlacePage.class,t);} public CompletableFuture<Models.PlacePage> districtPlacesAsync(String id,Map<String,?>q){return districtPlacesAsync(id,q,CancellationToken.none());}
  public CompletableFuture<Models.PlacePage> placesAsync(Map<String,?>q,CancellationToken t){return readAsync("/places",q,Models.PlacePage.class,t);} public CompletableFuture<Models.PlacePage> placesAsync(Map<String,?>q){return placesAsync(q,CancellationToken.none());}
  public CompletableFuture<Models.Place> placeAsync(String id,CancellationToken t){return readAsync("/places/"+segment(id),Map.of(),Models.Place.class,t);} public CompletableFuture<Models.Place> placeAsync(String id){return placeAsync(id,CancellationToken.none());}
  public CompletableFuture<Models.SearchPage> searchAsync(String q,Map<String,?>p,CancellationToken t){return readAsync("/search",with(p,"q",q),Models.SearchPage.class,t);} public CompletableFuture<Models.SearchPage> searchAsync(String q,Map<String,?>p){return searchAsync(q,p,CancellationToken.none());}
  public CompletableFuture<Models.SearchPage> autocompleteAsync(String q,int l,CancellationToken t){return readAsync("/autocomplete",Map.of("q",q,"limit",l),Models.SearchPage.class,t);} public CompletableFuture<Models.SearchPage> autocompleteAsync(String q,int l){return autocompleteAsync(q,l,CancellationToken.none());}
  public CompletableFuture<Models.SearchPage> geocodeAsync(String q,int l,CancellationToken t){return readAsync("/geocode",Map.of("q",q,"limit",l),Models.SearchPage.class,t);} public CompletableFuture<Models.SearchPage> geocodeAsync(String q,int l){return geocodeAsync(q,l,CancellationToken.none());}
  public CompletableFuture<Models.ReverseResult> reverseGeocodeAsync(double lat,double lng,CancellationToken t){return readAsync("/reverse",Map.of("lat",lat,"lng",lng),Models.ReverseResult.class,t);} public CompletableFuture<Models.ReverseResult> reverseGeocodeAsync(double lat,double lng){return reverseGeocodeAsync(lat,lng,CancellationToken.none());}
  public CompletableFuture<Models.PlacePage> nearbyAsync(double lat,double lng,int r,int l,CancellationToken t){return readAsync("/nearby",Map.of("lat",lat,"lng",lng,"radius",r,"limit",l),Models.PlacePage.class,t);} public CompletableFuture<Models.PlacePage> nearbyAsync(double lat,double lng,int r,int l){return nearbyAsync(lat,lng,r,l,CancellationToken.none());}
  public CompletableFuture<Models.BoundaryFeature> boundaryAsync(String id,CancellationToken t){return readAsync("/boundaries/"+segment(id),Map.of(),Models.BoundaryFeature.class,t);} public CompletableFuture<Models.BoundaryFeature> boundaryAsync(String id){return boundaryAsync(id,CancellationToken.none());}
  public CompletableFuture<Models.DatasetPage> datasetsAsync(CancellationToken t){return readAsync("/datasets",Map.of(),Models.DatasetPage.class,t);} public CompletableFuture<Models.DatasetPage> datasetsAsync(){return datasetsAsync(CancellationToken.none());}
  public CompletableFuture<Models.DownloadList> datasetDownloadsAsync(String v,CancellationToken t){return readAsync("/datasets/"+segment(v)+"/downloads",Map.of(),Models.DownloadList.class,t);} public CompletableFuture<Models.DownloadList> datasetDownloadsAsync(String v){return datasetDownloadsAsync(v,CancellationToken.none());}
  public CompletableFuture<Models.OpenData> roadsAsync(CancellationToken t){return getJsonAsync("/roads",Map.of(),t).thenApply(this::toOpenData);} public CompletableFuture<Models.OpenData> roadsAsync(){return roadsAsync(CancellationToken.none());}
  public CompletableFuture<Models.OpenData> pointsOfInterestAsync(CancellationToken t){return getJsonAsync("/pois",Map.of(),t).thenApply(this::toOpenData);} public CompletableFuture<Models.OpenData> pointsOfInterestAsync(){return pointsOfInterestAsync(CancellationToken.none());}

  public JsonNode getJson(String path,Map<String,?>q,CancellationToken t){return join(getJsonAsync(path,q,t));}
  public CompletableFuture<JsonNode> getJsonAsync(String path,Map<String,?>q,CancellationToken t){return attempt(request(path,q,"application/json"),path,0,t).thenApply(this::decodeJson);}
  private <T>T read(String p,Map<String,?>q,Class<T>c,CancellationToken t){return join(readAsync(p,q,c,t));}
  private <T>CompletableFuture<T> readAsync(String p,Map<String,?>q,Class<T>c,CancellationToken t){return getJsonAsync(p,q,t).thenApply(n->json.convertValue(n,c));}
  private <T>T join(CompletableFuture<T> f){try{return f.join();}catch(CompletionException e){if(e.getCause()instanceof RuntimeException r)throw r;throw e;}}
  private Models.OpenData openData(String p,CancellationToken t){return toOpenData(getJson(p,Map.of(),t));}
  private Models.OpenData toOpenData(JsonNode n){Map<String,JsonNode>m=new LinkedHashMap<>();n.fields().forEachRemaining(e->m.put(e.getKey(),e.getValue()));return new Models.OpenData(m);}

  private CompletableFuture<HttpResponse<WireBody>> attempt(HttpRequest request,String path,int attempt,CancellationToken token) {
    token.throwIfCancelled();
    long start = System.nanoTime();
    observe(new Models.WireEvidence("request",request.method(),request.uri().toString(),attempt,null,safeHeaders(request.headers().map()),0,null,null));
    CompletableFuture<HttpResponse<WireBody>> wire=http.sendAsync(request,boundedBody(maxJsonBytes));
    CompletableFuture<HttpResponse<WireBody>> result = new CompletableFuture<>();
    AutoCloseable registration=token.onCancel(()->{wire.cancel(true);result.cancel(true);});
    result.whenComplete((v,e)->{if(result.isCancelled())wire.cancel(true);try{registration.close();}catch(Exception ignored){}});
    wire.whenComplete((response, failure) -> {
      if (failure != null) {
        emit(path, attempt, null, start, null);
        if (failure instanceof CancellationException || attempt >= maxRetries) result.completeExceptionally(unwrap(failure));
        else delayed(retryMillis(attempt,null),token).thenCompose(x->attempt(request,path,attempt+1,token)).whenComplete(copyTo(result));
      } else {
        WireBody captured=response.body();observe(new Models.WireEvidence("response",request.method(),request.uri().toString(),attempt,response.statusCode(),safeHeaders(response.headers().map()),captured.size(),captured.sha256(),captured.preview()));
        emit(path, attempt, response.statusCode(), start, null);
        if (RETRYABLE.contains(response.statusCode()) && attempt < maxRetries)
          delayed(retryMillis(attempt,response),token).thenCompose(x->attempt(request,path,attempt+1,token)).whenComplete(copyTo(result));
        else result.complete(response);
      }
    });
    return result;
  }
  private CompletableFuture<Void> delayed(long millis,CancellationToken token){CompletableFuture<Void> f=new CompletableFuture<>();ScheduledFuture<?> scheduled=scheduler.schedule(()->f.complete(null),millis,TimeUnit.MILLISECONDS);AutoCloseable r=token.onCancel(()->{scheduled.cancel(true);f.cancel(true);});f.whenComplete((v,e)->{try{r.close();}catch(Exception ignored){}});return f;}
  private long retryMillis(int attempt,HttpResponse<?> response){String h=response==null?null:response.headers().firstValue("retry-after").orElse(null);long cap=Math.min(60_000L,retryDelay.toMillis()*(1L<<Math.min(attempt,20)));Long supplied=parseRetryAfter(h);long base=Math.min(60_000L,supplied==null?cap:supplied);return base==0?0:ThreadLocalRandom.current().nextLong(Math.max(1,base/2),base+1);}
  private static Long parseRetryAfter(String value){if(value==null)return null;try{return Math.max(0,Long.parseLong(value)*1000L);}catch(NumberFormatException ignored){try{return Math.max(0,java.time.ZonedDateTime.parse(value,java.time.format.DateTimeFormatter.RFC_1123_DATE_TIME).toInstant().toEpochMilli()-System.currentTimeMillis());}catch(RuntimeException invalid){return null;}}}
  private static <T> java.util.function.BiConsumer<T, Throwable> copyTo(CompletableFuture<T> target) { return (v, e) -> { if (e == null) target.complete(v); else target.completeExceptionally(unwrap(e)); }; }
  private JsonNode decodeJson(HttpResponse<WireBody> response) {
    byte[] body = response.body().bytes(); if (body.length > maxJsonBytes) throw new GhanaGeoException("Response exceeds configured JSON limit", response.statusCode(), "RESPONSE_TOO_LARGE", null, null, null);
    try {
      JsonNode root = body.length == 0 ? json.createObjectNode() : json.readTree(body);
      if (response.statusCode() < 200 || response.statusCode() >= 300) {
        JsonNode e = root.path("error"); throw new GhanaGeoException(e.path("message").asText("GhanaGeo request failed (" + response.statusCode() + ")"), response.statusCode(), text(e,"code"), text(e,"requestId"), text(e,"docs"), e.get("details"));
      }
      return root;
    } catch (IOException e) { throw new GhanaGeoException("Invalid JSON response", response.statusCode(), "INVALID_RESPONSE", null, null, null); }
  }

  private static HttpResponse.BodyHandler<WireBody> boundedBody(long maximum){return info->{long declared=info.headers().firstValueAsLong("content-length").orElse(-1);if(declared>maximum)return HttpResponse.BodySubscribers.mapping(HttpResponse.BodySubscribers.replacing(new byte[0]),v->{throw new CompletionException(new GhanaGeoException("Response exceeds configured limit",info.statusCode(),"RESPONSE_TOO_LARGE",null,null,null));});return new BoundedSubscriber(maximum);};}
  private record WireBody(byte[] bytes,long size,String sha256,String preview){WireBody{bytes=bytes.clone();}public byte[]bytes(){return bytes.clone();}}
  private static final class BoundedSubscriber implements HttpResponse.BodySubscriber<WireBody> {
    private final CompletableFuture<WireBody> body=new CompletableFuture<>();private final List<byte[]>chunks=new ArrayList<>();private final java.io.ByteArrayOutputStream preview=new java.io.ByteArrayOutputStream(512);private final MessageDigest digest;private final long maximum;private long received;private Flow.Subscription upstream;
    BoundedSubscriber(long maximum){this.maximum=maximum;}
    {try{digest=MessageDigest.getInstance("SHA-256");}catch(java.security.NoSuchAlgorithmException e){throw new ExceptionInInitializerError(e);}}
    public CompletionStage<WireBody> getBody(){return body;}
    public void onSubscribe(Flow.Subscription subscription){upstream=subscription;subscription.request(Long.MAX_VALUE);}
    public void onNext(List<ByteBuffer> items){for(ByteBuffer source:items){int size=source.remaining();received+=size;if(received>maximum){upstream.cancel();body.completeExceptionally(new GhanaGeoException("Response exceeds configured limit",0,"RESPONSE_TOO_LARGE",null,null,null));return;}byte[]copy=new byte[size];source.get(copy);digest.update(copy);if(preview.size()<512)preview.write(copy,0,Math.min(copy.length,512-preview.size()));chunks.add(copy);}}
    public void onError(Throwable error){body.completeExceptionally(error);} public void onComplete(){byte[]value=new byte[(int)received];int offset=0;for(byte[]chunk:chunks){System.arraycopy(chunk,0,value,offset,chunk.length);offset+=chunk.length;}body.complete(new WireBody(value,received,java.util.HexFormat.of().formatHex(digest.digest()),redactPreview(preview.toByteArray())));}
  }

  public Path downloadDatasetArtifact(String version,String entity,String format,Path target,Optional<String> checksum,CancellationToken token){return join(downloadDatasetArtifactAsync(version,entity,format,target,checksum,token));}
  public Path downloadDatasetArtifact(String version,String entity,String format,Path target,Optional<String> checksum){return downloadDatasetArtifact(version,entity,format,target,checksum,CancellationToken.none());}
  public CompletableFuture<Path> downloadDatasetArtifactAsync(String version,String entity,String format,Path target,Optional<String> checksum,CancellationToken token){
    String path = "/datasets/" + segment(version) + "/downloads/" + segment(entity) + "." + segment(format);
    return artifactAttempt(request(path,Map.of(),"application/octet-stream"),path,0,target,checksum,token);
  }
  public CompletableFuture<Path> downloadDatasetArtifactAsync(String version,String entity,String format,Path target,Optional<String> checksum){return downloadDatasetArtifactAsync(version,entity,format,target,checksum,CancellationToken.none());}
  private CompletableFuture<Path> artifactAttempt(HttpRequest request,String path,int attempt,Path target,Optional<String> expectedSha256,CancellationToken token){
    token.throwIfCancelled();observe(new Models.WireEvidence("request",request.method(),request.uri().toString(),attempt,null,safeHeaders(request.headers().map()),0,null,null));CompletableFuture<HttpResponse<java.io.InputStream>> wire=http.sendAsync(request,HttpResponse.BodyHandlers.ofInputStream());CompletableFuture<Path> out=new CompletableFuture<>();AutoCloseable reg=token.onCancel(()->{wire.cancel(true);out.cancel(true);});out.whenComplete((v,e)->{try{reg.close();}catch(Exception ignored){}});
    wire.whenComplete((response,failure)->{if(failure!=null){if(attempt<maxRetries&&!token.isCancelled())delayed(retryMillis(attempt,null),token).thenCompose(x->artifactAttempt(request,path,attempt+1,target,expectedSha256,token)).whenComplete(copyTo(out));else out.completeExceptionally(unwrap(failure));return;}if(RETRYABLE.contains(response.statusCode())&&attempt<maxRetries){try{response.body().close();}catch(IOException ignored){}delayed(retryMillis(attempt,response),token).thenCompose(x->artifactAttempt(request,path,attempt+1,target,expectedSha256,token)).whenComplete(copyTo(out));return;}CompletableFuture.runAsync(()->copyArtifact(request,attempt,response,target,expectedSha256,token)).whenComplete((v,e)->{if(e==null)out.complete(target);else out.completeExceptionally(unwrap(e));});});return out;
  }
  private void copyArtifact(HttpRequest request,int attempt,HttpResponse<java.io.InputStream> response,Path target,Optional<String> expectedSha256,CancellationToken token){
    if (response.statusCode() < 200 || response.statusCode() >= 300) { try (var in = response.body()) { byte[] bytes = in.readNBytes(maxJsonBytes + 1);observe(new Models.WireEvidence("response",request.method(),request.uri().toString(),attempt,response.statusCode(),safeHeaders(response.headers().map()),bytes.length,sha256(bytes),redactPreview(bytes)));failDownload(response.statusCode(), bytes); return; } catch (IOException e) { throw new CompletionException(e); } }
    try (var in = response.body(); var out = Files.newOutputStream(target)) {
      MessageDigest digest = MessageDigest.getInstance("SHA-256"); byte[] buffer = new byte[8192]; long total = 0;
      for (int n; (n = in.read(buffer)) >= 0;) { token.throwIfCancelled();total += n; if (total > maxArtifactBytes) throw new GhanaGeoException("Artifact exceeds configured limit", response.statusCode(), "RESPONSE_TOO_LARGE", null, null, null); out.write(buffer, 0, n); digest.update(buffer, 0, n); }
      String actual = java.util.HexFormat.of().formatHex(digest.digest());observe(new Models.WireEvidence("response",request.method(),request.uri().toString(),attempt,response.statusCode(),safeHeaders(response.headers().map()),total,actual,"[binary artifact]"));if (expectedSha256.isPresent() && !actual.equalsIgnoreCase(expectedSha256.get())) throw new GhanaGeoException("Dataset checksum mismatch", 422, "CHECKSUM_MISMATCH", null, null, null);
    } catch (GhanaGeoException e) { try { Files.deleteIfExists(target); } catch (IOException ignored) {} throw e; }
    catch (Exception e) { try { Files.deleteIfExists(target); } catch (IOException ignored) {} throw new CompletionException(e); }
  }
  private void failDownload(int status,byte[] bytes){decodeJson(new SimpleResponse(status,new WireBody(bytes,bytes.length,sha256(bytes),redactPreview(bytes))));throw new AssertionError();}

  public Iterable<Models.PlacePage> placePages(Map<String,?>q,CancellationToken t){return pages(c->places(with(q,"cursor",c),t));}
  public Iterable<Models.PlacePage> placePages(Map<String,?>q){return placePages(q,CancellationToken.none());}
  public Iterable<Models.RegionPage> regionPages(Map<String,?>q,CancellationToken t){return pages(c->regions(with(q,"cursor",c),t));}
  public Iterable<Models.RegionPage> regionPages(Map<String,?>q){return regionPages(q,CancellationToken.none());}
  public Iterable<Models.DistrictPage> districtPages(Map<String,?>q,CancellationToken t){return pages(c->districts(with(q,"cursor",c),t));}
  public Iterable<Models.DistrictPage> districtPages(Map<String,?>q){return districtPages(q,CancellationToken.none());}
  private <P extends Models.Page<?>>Iterable<P> pages(Function<String,P> fetch){return()->new java.util.Iterator<>(){String cursor;P next;boolean finished;final Set<String>seen=new HashSet<>();public boolean hasNext(){if(next==null&&!finished){next=fetch.apply(cursor);String value=next.nextCursor();finished=value==null||value.isBlank();if(!finished&&!seen.add(value))throw new GhanaGeoException("Repeated pagination cursor",500,"INVALID_CURSOR",null,null,null);cursor=value;}return next!=null;}public P next(){if(!hasNext())throw new java.util.NoSuchElementException();P v=next;next=null;return v;}};}
  public Flow.Publisher<Models.PlacePage> placePagePublisher(Map<String,?>q,CancellationToken t){return new PagePublisher<>((c,ct)->placesAsync(with(q,"cursor",c),ct),t);}
  public Flow.Publisher<Models.RegionPage> regionPagePublisher(Map<String,?>q,CancellationToken t){return new PagePublisher<>((c,ct)->regionsAsync(with(q,"cursor",c),ct),t);}
  public Flow.Publisher<Models.DistrictPage> districtPagePublisher(Map<String,?>q,CancellationToken t){return new PagePublisher<>((c,ct)->districtsAsync(with(q,"cursor",c),ct),t);}
  private static final class PagePublisher<P extends Models.Page<?>> implements Flow.Publisher<P>{private final java.util.function.BiFunction<String,CancellationToken,CompletableFuture<P>>fetch;private final CancellationToken parent;PagePublisher(java.util.function.BiFunction<String,CancellationToken,CompletableFuture<P>>f,CancellationToken t){fetch=f;parent=t;}public void subscribe(Flow.Subscriber<? super P>s){java.util.Objects.requireNonNull(s);CancellationToken.Source source=CancellationToken.source();parent.onCancel(source::cancel);s.onSubscribe(new Flow.Subscription(){long demand;String cursor;boolean active,done;final Set<String>seen=new HashSet<>();public synchronized void request(long n){if(n<=0){done=true;source.cancel();s.onError(new IllegalArgumentException("demand must be positive"));return;}demand=demand>Long.MAX_VALUE-n?Long.MAX_VALUE:demand+n;pump();}public synchronized void cancel(){done=true;source.cancel();}private synchronized void pump(){if(done||active||demand==0)return;if(source.token().isCancelled()){done=true;s.onError(new CancellationException());return;}active=true;fetch.apply(cursor,source.token()).whenComplete((page,error)->{synchronized(this){active=false;if(done)return;if(error!=null){done=true;s.onError(unwrap(error));return;}demand--;String next=page.nextCursor();if(next!=null&&!next.isBlank()&&!seen.add(next)){done=true;source.cancel();s.onError(new GhanaGeoException("Repeated pagination cursor",500,"INVALID_CURSOR",null,null,null));return;}cursor=next;s.onNext(page);if(next==null||next.isBlank()){done=true;source.close();s.onComplete();}else pump();}});}});}}

  private HttpRequest request(String path, Map<String, ?> query, String accept) { HttpRequest.Builder b = HttpRequest.newBuilder(uri(path, query)).timeout(Duration.ofSeconds(30)).header("Accept", accept).header("User-Agent", "ghanageo-java/" + Version.SDK).GET(); defaultHeaders.forEach(b::header); if (apiKey != null && !apiKey.isBlank()) b.header("Authorization", "Bearer " + apiKey); return b.build(); }
  private URI uri(String path, Map<String, ?> query) { StringBuilder u = new StringBuilder(baseUri.toString()).append(path); boolean first = true; for (var e : query.entrySet()) if (e.getValue() != null && !String.valueOf(e.getValue()).isBlank()) { u.append(first?'?':'&').append(encoded(e.getKey())).append('=').append(encoded(String.valueOf(e.getValue()))); first=false; } return URI.create(u.toString()); }
  private void emit(String path,int attempt,Integer status,long start,String code) { if (telemetry != null) try { telemetry.accept(new Models.TelemetryEvent(path,"GET",attempt,status,(System.nanoTime()-start)/1_000_000,code)); } catch (RuntimeException ignored) {} }
  private void observe(Models.WireEvidence evidence){if(wireObserver!=null)try{wireObserver.accept(evidence);}catch(RuntimeException ignored){}}
  private static Map<String,List<String>> safeHeaders(Map<String,List<String>> input){Map<String,List<String>>out=new LinkedHashMap<>();input.forEach((k,v)->out.put(k,k.equalsIgnoreCase("authorization")?List.of("[REDACTED]"):List.copyOf(v)));return out;}
  private static String sha256(byte[]value){try{return java.util.HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(value));}catch(java.security.NoSuchAlgorithmException e){throw new IllegalStateException(e);}}
  private static String redactPreview(byte[]value){int length=Math.min(512,value.length);String text=new String(value,0,length,StandardCharsets.UTF_8);return text.replaceAll("(?i)Bearer\\s+[A-Za-z0-9._-]+","Bearer [REDACTED]").replaceAll("gh_(?:live|test)_[A-Za-z0-9_-]+","[REDACTED]");}
  private static Throwable unwrap(Throwable e) { return e instanceof CompletionException && e.getCause()!=null ? e.getCause() : e; }
  private static String stripSlash(String s) { return s.endsWith("/") ? s.substring(0,s.length()-1) : s; }
  private static String encoded(String s) { return URLEncoder.encode(s, StandardCharsets.UTF_8).replace("+", "%20"); }
  private static String segment(String s) { return encoded(java.util.Objects.requireNonNull(s)); }
  private static String text(JsonNode n,String key) { JsonNode v=n.get(key); return v==null||v.isNull()||v.asText().isEmpty()?null:v.asText(); }
  private static Map<String,Object> with(Map<String,?> map,String key,Object value) { Map<String,Object> copy=new LinkedHashMap<>(map); if(value!=null) copy.put(key,value); return copy; }
  public void close(){scheduler.shutdownNow();}

  /** Client configuration. API keys and telemetry are opt-in. */
  public static final class Builder {
    private String baseUrl=DEFAULT_BASE_URL, apiKey; private HttpClient httpClient=HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(10)).build(); private ObjectMapper objectMapper=new ObjectMapper(); private int maxRetries=2,maxJsonBytes=5*1024*1024; private long maxArtifactBytes=100L*1024*1024; private Duration retryDelay=Duration.ofMillis(100); private Consumer<Models.TelemetryEvent> telemetry;private Consumer<Models.WireEvidence> wireObserver; private final Map<String,String> defaultHeaders=new LinkedHashMap<>();
    public Builder baseUrl(String v){baseUrl=v;return this;} public Builder apiKey(String v){apiKey=v;return this;} public Builder httpClient(HttpClient v){httpClient=v;return this;} public Builder objectMapper(ObjectMapper v){objectMapper=v;return this;} public Builder maxRetries(int v){maxRetries=v;return this;} public Builder retryDelay(Duration v){retryDelay=v;return this;} public Builder maxJsonBytes(int v){maxJsonBytes=v;return this;} public Builder maxArtifactBytes(long v){maxArtifactBytes=v;return this;} public Builder telemetry(Consumer<Models.TelemetryEvent> v){telemetry=v;return this;}public Builder wireObserver(Consumer<Models.WireEvidence>v){wireObserver=v;return this;} public Builder defaultHeader(String k,String v){defaultHeaders.put(k,v);return this;} public GhanaGeoClient build(){return new GhanaGeoClient(this);}
  }

  private record SimpleResponse(int statusCode,WireBody body) implements HttpResponse<WireBody> { public HttpRequest request(){throw new UnsupportedOperationException();} public Optional<HttpResponse<WireBody>> previousResponse(){return Optional.empty();} public java.net.http.HttpHeaders headers(){return java.net.http.HttpHeaders.of(Map.of(),(a,b)->true);} public URI uri(){return URI.create("http://localhost");} public HttpClient.Version version(){return HttpClient.Version.HTTP_1_1;} public Optional<javax.net.ssl.SSLSession> sslSession(){return Optional.empty();} }
}
