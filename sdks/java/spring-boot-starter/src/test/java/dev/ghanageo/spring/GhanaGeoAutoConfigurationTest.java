package dev.ghanageo.spring;
import dev.ghanageo.GhanaGeoClient;
import org.junit.jupiter.api.Test;
import org.springframework.boot.test.context.runner.ApplicationContextRunner;
import static org.assertj.core.api.Assertions.assertThat;
final class GhanaGeoAutoConfigurationTest { @Test void suppliesConfiguredClient(){new ApplicationContextRunner().withUserConfiguration(GhanaGeoAutoConfiguration.class).withPropertyValues("ghanageo.base-url=http://localhost:8180/v1").run(c->{assertThat(c).hasSingleBean(GhanaGeoClient.class);assertThat(c.getBean(GhanaGeoClient.class).baseUri().toString()).isEqualTo("http://localhost:8180/v1");});} }
