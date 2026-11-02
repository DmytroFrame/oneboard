// Package modules is the registry of the application's fx modules, grouped by namespace.
//
// Every package registers its own module where it is declared, e.g.
//
//	var Module = modules.Register(modules.Adapters, fx.Module("postgres", ...))
//
// and the composition root assembles the application from whole namespaces (modules.All), without
// listing packages. Registration happens during package initialisation, so a package only takes part
// if it is linked into the binary: internal/app imports every one of them for that reason.
package modules

import (
	"fmt"
	"sync"

	"go.uber.org/fx"
)

// Namespace groups modules of one architectural layer.
type Namespace string

const (
	Core      Namespace = "core"        // configuration and other process-wide basics
	Adapters  Namespace = "adapters"    // infrastructure, each binding itself to the ports it implements
	UseCases  Namespace = "usecases"    // business logic
	Transport Namespace = "transport"   // delivery: the HTTP server, middleware, presenters
	Routes    Namespace = "http.routes" // HTTP handlers; each contributes its endpoints to the router
)

// Order is the order in which All assembles the namespaces. fx resolves dependencies by type, so the
// order only affects how the DI graph is printed.
var Order = []Namespace{Core, Adapters, UseCases, Transport, Routes}

var (
	mu       sync.Mutex
	registry = map[Namespace][]fx.Option{}
	frozen   bool
)

// Register adds a module to a namespace and returns it, so it can also be kept as the package's Module.
// Registering after the application has been assembled panics: the module would silently be left out.
func Register(ns Namespace, module fx.Option) fx.Option {
	mu.Lock()
	defer mu.Unlock()
	if frozen {
		panic(fmt.Sprintf("modules: %q registered after the application was assembled", ns))
	}
	registry[ns] = append(registry[ns], module)
	return module
}

// Of returns the modules registered in a namespace, in registration order.
func Of(ns Namespace) []fx.Option {
	mu.Lock()
	defer mu.Unlock()
	return append([]fx.Option(nil), registry[ns]...)
}

// All returns every registered module of the given namespaces (Order when none are given), and closes
// the registry for further registrations.
func All(namespaces ...Namespace) fx.Option {
	if len(namespaces) == 0 {
		namespaces = Order
	}
	mu.Lock()
	frozen = true
	mu.Unlock()
	var opts []fx.Option
	for _, ns := range namespaces {
		opts = append(opts, Of(ns)...)
	}
	return fx.Options(opts...)
}
