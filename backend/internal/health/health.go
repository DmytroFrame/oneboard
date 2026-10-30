// Package health collects readiness checks from the adapters (fx group "health"), so the load
// balancer only routes traffic to an instance whose critical dependencies answer.
package health

import (
	"context"

	"go.uber.org/fx"
)

type Check struct {
	Name string
	Run  func(ctx context.Context) error
}

// Provide registers a constructor returning a Check.
func Provide(constructor any) fx.Option {
	return fx.Provide(fx.Annotate(constructor, fx.ResultTags(`group:"health"`)))
}
