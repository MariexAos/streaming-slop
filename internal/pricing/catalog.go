// Package pricing owns provider rate cards and reproducible CNY estimates.
package pricing

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"
)

//go:embed catalog.json
var catalogJSON []byte

var defaultCatalog = sync.OnceValues(func() (Catalog, error) { return Parse(catalogJSON) })

type Promotion struct {
	Price     float64    `json:"price"`
	ExpiresAt *time.Time `json:"expiresAt"`
	Note      string     `json:"note"`
}
type Rate struct {
	Provider      string     `json:"provider"`
	Model         string     `json:"model"`
	Variant       string     `json:"variant"`
	BillingUnit   string     `json:"billingUnit"`
	UnitQuantity  int64      `json:"unitQuantity"`
	Currency      string     `json:"currency"`
	StandardPrice float64    `json:"standardPrice"`
	Promotion     *Promotion `json:"promotion"`
	ReserveAt     string     `json:"reserveAt"`
	Source        string     `json:"source"`
}
type Catalog struct {
	Version            string             `json:"version"`
	ExchangeRatesToCNY map[string]float64 `json:"exchangeRatesToCny"`
	ExchangeRateAsOf   string             `json:"exchangeRateAsOf"`
	Rates              []Rate             `json:"rates"`
}
type Quote struct {
	Rate
	Version           string    `json:"version"`
	QuotedAt          time.Time `json:"quotedAt"`
	ExchangeRateToCNY float64   `json:"exchangeRateToCny"`
	ExchangeRateAsOf  string    `json:"exchangeRateAsOf"`
	PromotionActive   bool      `json:"promotionActive"`
	StandardCNY       float64   `json:"standardCny"`
	EstimatedCNY      float64   `json:"estimatedCny"`
	ReserveCNY        float64   `json:"reserveCny"`
}

func Parse(data []byte) (Catalog, error) {
	var c Catalog
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&c); err != nil {
		return c, fmt.Errorf("decode pricing catalog: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return c, fmt.Errorf("pricing catalog must contain one JSON object")
	}
	if c.Version == "" || c.ExchangeRateAsOf == "" || len(c.Rates) == 0 || c.ExchangeRatesToCNY["CNY"] != 1 {
		return c, fmt.Errorf("pricing catalog metadata missing or invalid")
	}
	seen := make(map[string]bool)
	for _, r := range c.Rates {
		if err := validateRate(r, c.ExchangeRatesToCNY[r.Currency]); err != nil {
			return c, err
		}
		key := r.Provider + "/" + r.Model + "/" + r.Variant
		if seen[key] {
			return c, fmt.Errorf("duplicate price: %s", key)
		}
		seen[key] = true
	}
	return c, nil
}
func validateRate(r Rate, exchange float64) error {
	if r.Provider == "" || r.Model == "" || r.Variant == "" || r.Source == "" || r.StandardPrice <= 0 || r.UnitQuantity <= 0 || exchange <= 0 {
		return fmt.Errorf("invalid price for %s/%s/%s", r.Provider, r.Model, r.Variant)
	}
	switch r.BillingUnit {
	case "output_second", "input_token", "output_token", "speech_character":
	default:
		return fmt.Errorf("unsupported billing unit %q", r.BillingUnit)
	}
	if r.ReserveAt != "standard" && r.ReserveAt != "effective" {
		return fmt.Errorf("unsupported reservation basis %q", r.ReserveAt)
	}
	if r.Promotion != nil && (r.Promotion.Price <= 0 || r.Promotion.Price > r.StandardPrice || r.Promotion.Note == "") {
		return fmt.Errorf("invalid promotion for %s", r.Model)
	}
	return nil
}
func Lookup(provider, model, variant string, at time.Time) (Quote, error) {
	c, err := defaultCatalog()
	if err != nil {
		return Quote{}, err
	}
	return c.Lookup(provider, model, variant, at)
}
func (c Catalog) Lookup(provider, model, variant string, at time.Time) (Quote, error) {
	for _, r := range c.Rates {
		if r.Provider != provider || r.Model != model || r.Variant != variant {
			continue
		}
		fx := c.ExchangeRatesToCNY[r.Currency]
		q := Quote{Rate: r, Version: c.Version, QuotedAt: at, ExchangeRateToCNY: fx, ExchangeRateAsOf: c.ExchangeRateAsOf, StandardCNY: r.StandardPrice * fx}
		q.EstimatedCNY = q.StandardCNY
		if r.Promotion != nil && (r.Promotion.ExpiresAt == nil || at.Before(*r.Promotion.ExpiresAt)) {
			q.PromotionActive = true
			q.EstimatedCNY = r.Promotion.Price * fx
		}
		q.ReserveCNY = q.EstimatedCNY
		if r.ReserveAt == "standard" {
			q.ReserveCNY = q.StandardCNY
		}
		return q, nil
	}
	return Quote{}, fmt.Errorf("no price for %s/%s/%s", provider, model, variant)
}
func (q Quote) Estimate(quantity float64) float64 {
	return quantity * q.EstimatedCNY / float64(q.UnitQuantity)
}
func (q Quote) Reserve(quantity float64) float64 {
	return quantity * q.ReserveCNY / float64(q.UnitQuantity)
}
