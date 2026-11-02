package modules

import (
	"testing"

	"go.uber.org/fx"
)

func TestRegisterAndAssemble(t *testing.T) {
	type a struct{}
	type b struct{}
	Register(Adapters, fx.Provide(func() a { return a{} }))
	Register(UseCases, fx.Provide(func(a) b { return b{} }))
	if len(Of(Adapters)) != 1 || len(Of(UseCases)) != 1 || len(Of(Routes)) != 0 {
		t.Fatal("modules ended up in the wrong namespaces")
	}
	if err := fx.ValidateApp(All(), fx.Invoke(func(b) {})); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("registering after assembly must panic, or the module would silently be missing")
		}
	}()
	Register(Adapters, fx.Options())
}
