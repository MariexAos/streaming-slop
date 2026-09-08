package generation

import (
	"context"
	"errors"
	"fmt"
	"math"
)

var ErrBudgetExceeded = errors.New("generation budget exhausted")

type Budget interface {
	Reserve(context.Context, string, int64) error
	Settle(context.Context, string, int64) error
}

func Micros(cny float64) int64 { return int64(math.Ceil(cny * 1_000_000)) }

// Budgeted reserves the full quote before the first external side effect.
// Unknown outcomes retain their reservation until reconciliation.
type Budgeted struct {
	AllowUnsettledCost bool
	Generator
	Budget Budget
}

func (b Budgeted) BuildRequest(r Request) (Request, error) {
	builder, ok := b.Generator.(RequestBuilder)
	if !ok {
		return r, errors.New("provider does not support cost quotes")
	}
	request, err := builder.BuildRequest(r)
	if err != nil {
		return request, err
	}
	if request.UnitPriceCNY == nil || *request.UnitPriceCNY <= 0 {
		return request, errors.New("provider has no positive CNY quote")
	}
	return request, nil
}

func (b Budgeted) Submit(ctx context.Context, r Request) (Job, error) {
	if r.UnitPriceCNY == nil || *r.UnitPriceCNY <= 0 {
		return Job{}, errors.New("a positive CNY quote is required before generation")
	}
	if err := b.Budget.Reserve(ctx, string(r.AttemptID), Micros(*r.UnitPriceCNY*r.Duration.Seconds())); err != nil {
		return Job{}, err
	}
	return b.Generator.Submit(ctx, r)
}

func (b Budgeted) ReconcileCost(ctx context.Context, attemptID string, cost *float64) error {
	if cost == nil && b.AllowUnsettledCost {
		return nil
	}
	if cost == nil {
		return fmt.Errorf("missing actual cost for attempt %s", attemptID)
	}
	return b.Budget.Settle(ctx, attemptID, Micros(*cost))
}
