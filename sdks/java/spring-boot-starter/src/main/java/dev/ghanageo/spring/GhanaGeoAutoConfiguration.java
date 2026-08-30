package dev.ghanageo.spring;

import dev.ghanageo.GhanaGeoClient;
import org.springframework.boot.autoconfigure.AutoConfiguration;
import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
import org.springframework.boot.context.properties.EnableConfigurationProperties;
import org.springframework.context.annotation.Bean;

/** Creates a GhanaGeo client when an application has not supplied one. */
@AutoConfiguration
@EnableConfigurationProperties(GhanaGeoProperties.class)
public class GhanaGeoAutoConfiguration {
  @Bean @ConditionalOnMissingBean public GhanaGeoClient ghanaGeoClient(GhanaGeoProperties p){var b=GhanaGeoClient.builder().baseUrl(p.getBaseUrl()).maxRetries(p.getMaxRetries()).retryDelay(p.getRetryDelay()).maxJsonBytes(p.getMaxJsonBytes()).maxArtifactBytes(p.getMaxArtifactBytes());if(p.getApiKey()!=null&&!p.getApiKey().isBlank())b.apiKey(p.getApiKey());return b.build();}
}
