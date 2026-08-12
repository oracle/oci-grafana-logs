package plugin

import "testing"

func TestCustomLoggingSearchEndpoint(t *testing.T) {
	got := customEndpoint("logging-search", "eu-budapest-1", "drcc1.gov.hu")
	want := "https://logging-search.eu-budapest-1.drcc1.gov.hu"

	if got != want {
		t.Fatalf("customEndpoint() = %q, want %q", got, want)
	}
}
