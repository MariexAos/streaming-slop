package pricing

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

func TestPromotionExpiryAndReservation(t *testing.T) {
	cutoff := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		at     time.Time
		active bool
		price  float64
	}{
		{"before", cutoff.Add(-time.Nanosecond), true, 0.0419375},
		{"at", cutoff, false, 0.16775},
		{"after", cutoff.Add(time.Hour), false, 0.16775},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q, err := Lookup("fal", "minimax/h3-max-turbo/image-to-video", "480P", tc.at)
			if err != nil {
				t.Fatal(err)
			}
			if q.PromotionActive != tc.active || math.Abs(q.Estimate(1)-tc.price) > 1e-10 || math.Abs(q.Reserve(5)-5*tc.price) > 1e-10 {
				t.Fatalf("incorrect quote: %+v", q)
			}
		})
	}
}
func TestCatalogUnitsAndUnknownPrices(t *testing.T) {
	at := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		model, variant string
		quantity, want float64
	}{
		{"MiniMax-H3-Max", "480P", 5, 1.65},
		{"MiniMax-H3-Max", "768P", 5, 2.5},
		{"MiniMax-M3", "standard-input-up-to-512k", 1000000, 2.1},
		{"MiniMax-M3", "standard-output-up-to-512k", 1000000, 8.4},
		{"speech-2.8-turbo", "standard", 10000, 2},
	} {
		q, err := Lookup("minimax", tc.model, tc.variant, at)
		if err != nil {
			t.Fatal(err)
		}
		if math.Abs(q.Estimate(tc.quantity)-tc.want) > 1e-10 {
			t.Fatalf("bad unit conversion for %s", tc.model)
		}
	}
	if _, err := Lookup("unknown", "MiniMax-H3-Max", "480P", at); err == nil {
		t.Fatal("unknown provider must not have a quote")
	}
}
func TestRejectInvalidCatalog(t *testing.T) {
	for _, mutate := range []func(*Catalog){
		func(c *Catalog) { c.Rates = append(c.Rates, c.Rates[0]) },
		func(c *Catalog) { c.Rates[0].UnitQuantity = 0 },
		func(c *Catalog) { c.Rates[0].Currency = "unknown" },
		func(c *Catalog) { c.Rates[0].StandardPrice = -1 },
		func(c *Catalog) { c.Rates[0].BillingUnit = "unknown" },
	} {
		c, err := Parse(catalogJSON)
		if err != nil {
			t.Fatal(err)
		}
		mutate(&c)
		data, err := json.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Parse(data); err == nil {
			t.Fatal("invalid catalog accepted")
		}
	}
}
