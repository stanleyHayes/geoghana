package dev.ghanageo.spring;

import java.time.Duration;
import org.springframework.boot.context.properties.ConfigurationProperties;

/** GhanaGeo client settings. No API key is required for public reads. */
@ConfigurationProperties("ghanageo")
public class GhanaGeoProperties {
  private String baseUrl="https://api.geo.digitalghana.dev/v1"; private String apiKey; private int maxRetries=2; private Duration retryDelay=Duration.ofMillis(100); private int maxJsonBytes=5*1024*1024; private long maxArtifactBytes=100L*1024*1024;
  public String getBaseUrl(){return baseUrl;} public void setBaseUrl(String v){baseUrl=v;} public String getApiKey(){return apiKey;} public void setApiKey(String v){apiKey=v;} public int getMaxRetries(){return maxRetries;} public void setMaxRetries(int v){maxRetries=v;} public Duration getRetryDelay(){return retryDelay;} public void setRetryDelay(Duration v){retryDelay=v;} public int getMaxJsonBytes(){return maxJsonBytes;} public void setMaxJsonBytes(int v){maxJsonBytes=v;} public long getMaxArtifactBytes(){return maxArtifactBytes;} public void setMaxArtifactBytes(long v){maxArtifactBytes=v;}
}
