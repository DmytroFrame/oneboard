package app

import (
	"testing"

	"go.uber.org/fx"

	"github.com/DmytroFrame/oneboard/backend/internal/modules"
)

// The dependency graph is complete: every port has a binding, nothing is provided twice, and every
// registered route handler can be built. Constructors are not run, so no infrastructure is needed.
func TestDependencyGraph(t *testing.T) {
	if err := fx.ValidateApp(Options()); err != nil {
		t.Fatal(err)
	}
}

func TestEveryNamespaceHasModules(t *testing.T) {
	for _, ns := range modules.Order {
		if len(modules.Of(ns)) == 0 {
			t.Errorf("nothing registered in %q (is the package imported in link.go?)", ns)
		}
	}
	if n := len(modules.Of(modules.Routes)); n != 10 {
		t.Errorf("%d route handlers registered, want 10", n)
	}
}
