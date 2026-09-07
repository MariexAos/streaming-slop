package main

import (
	"strings"
	"testing"
)

func TestIntegrationReportRejectsMissingSkippedAndFailedTests(t *testing.T) {
	for _, report := range []string{"", `{"Action":"skip","Package":"p","Test":"TestDatabase"}`, `{"Action":"fail","Package":"p"}`, `not json`} {
		if err := integrationResult(strings.NewReader(report)); err == nil {
			t.Fatalf("accepted invalid report %q", report)
		}
	}
}
