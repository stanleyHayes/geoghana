package dev.ghanageo;

import com.fasterxml.jackson.databind.JsonNode;

/** A structured GhanaGeo API failure. */
public final class GhanaGeoException extends RuntimeException {
  private static final long serialVersionUID = 1L;
  private final int status;
  private final String code;
  private final String requestId;
  private final String docs;
  private final transient JsonNode details;
  private final transient Models.ErrorBody error;

  public GhanaGeoException(String message, int status, String code, String requestId, String docs, JsonNode details) {
    super(message); this.status=status;this.code=code;this.requestId=requestId;this.docs=docs;this.details=details;java.util.Map<String,JsonNode> values=new java.util.LinkedHashMap<>();if(details!=null&&details.isObject())details.fields().forEachRemaining(e->values.put(e.getKey(),e.getValue()));this.error=new Models.ErrorBody(code,message,requestId,values,docs);
  }
  public int status() { return status; }
  public String code() { return code; }
  public String requestId() { return requestId; }
  public String docs() { return docs; }
  public JsonNode details() { return details; }
  public Models.ErrorBody error(){return error;}
}
