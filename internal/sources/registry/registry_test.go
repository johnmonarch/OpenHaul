package registry

import "testing"

func TestCompanyCensusDatasetRegistered(t *testing.T) {
	dataset, ok := DatasetByKey(CompanyCensusKey)
	if !ok {
		t.Fatal("company census dataset is not registered")
	}
	if dataset.ID != "az4n-8mr2" {
		t.Fatalf("dataset ID = %q", dataset.ID)
	}
	if dataset.SourceName != "dot_datahub_socrata" {
		t.Fatalf("source name = %q", dataset.SourceName)
	}
	if dataset.ResourceURL != "https://data.transportation.gov/resource/az4n-8mr2.json" {
		t.Fatalf("resource URL = %q", dataset.ResourceURL)
	}
}
