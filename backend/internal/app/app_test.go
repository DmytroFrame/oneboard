package app

import (
	"os"
	"path/filepath"
	"strings"
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
	// Every handler file registers itself with registerRoutes; all of them must have run.
	files, _ := filepath.Glob("../transport/httpapi/*.go")
	want := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, _ := os.ReadFile(f)
		want += strings.Count(string(src), "registerRoutes(New")
	}
	if n := len(modules.Of(modules.Routes)); n != want || want == 0 {
		t.Errorf("%d route handlers registered, want %d", n, want)
	}
}
