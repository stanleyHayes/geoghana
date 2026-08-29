package geonames

import (
	"strings"
	"testing"
)

func TestSourceRecordCarriesTraceablePayloadMetadata(t *testing.T) {
	row := make([]string, minColumns)
	row[colGeonameID] = "2306104"
	row[colName] = "Accra"
	row[colLatitude] = "5.55602"
	row[colLongitude] = "-0.1969"
	row[colFeatureCls] = "P"
	row[colFeatureCode] = "PPLC"
	row[colCountry] = "GH"
	row[colAdmin1] = "01"

	adapter := &Adapter{RetrievedAt: "2026-08-29T12:00:00Z"}
	record, ok := adapter.toRecord(row)
	if !ok {
		t.Fatal("valid GeoNames row was rejected")
	}
	if record.Provenance.ExternalID != row[colGeonameID] || record.Provenance.RetrievedAt != adapter.RetrievedAt {
		t.Fatalf("source identity was lost: %+v", record.Provenance)
	}
	hash := record.Provenance.SourcePayloadHash
	if len(hash) != 64 || strings.Trim(hash, "0123456789abcdef") != "" {
		t.Fatalf("payload hash is not lowercase SHA-256: %q", hash)
	}
	row[colName] = "Accra changed"
	changed, ok := adapter.toRecord(row)
	if !ok || changed.Provenance.SourcePayloadHash == hash {
		t.Fatal("payload changes did not change the provenance hash")
	}
}
