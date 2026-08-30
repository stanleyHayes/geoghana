package example;
import dev.ghanageo.GhanaGeoClient;
import java.util.Map;
public final class Consumer { private Consumer(){} public static void main(String[]args){try(var client=GhanaGeoClient.create()){client.regions(Map.of("limit",1));}} }
