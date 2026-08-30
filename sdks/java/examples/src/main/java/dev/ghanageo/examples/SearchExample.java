package dev.ghanageo.examples;
import dev.ghanageo.GhanaGeoClient;
import java.util.Map;
public final class SearchExample { private SearchExample(){} public static void main(String[]args){try(var client=GhanaGeoClient.create()){client.search("Kumasi",Map.of("limit",5)).data().forEach(hit->System.out.println(hit.name()));}} }
