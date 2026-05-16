package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/luxview/engine/internal/model"
)

type fakeSourceCheckout struct {
	name   string
	called bool
}

func (c *fakeSourceCheckout) Checkout(_ context.Context, _ *model.App, _ string, destDir string) (*model.CheckoutResult, error) {
	c.called = true
	return &model.CheckoutResult{WorkDir: destDir, Ref: c.name}, nil
}

func TestAppSourceCheckoutUsesLuxViewWhenRepositoryIDExists(t *testing.T) {
	legacy := &fakeSourceCheckout{name: "legacy"}
	luxview := &fakeSourceCheckout{name: "luxview"}
	checkout := NewAppSourceCheckout(legacy, luxview)
	repositoryID := uuid.New()

	result, err := checkout.Checkout(context.Background(), &model.App{RepositoryID: &repositoryID}, "main", t.TempDir())
	if err != nil {
		t.Fatalf("Checkout() error = %v", err)
	}
	if !luxview.called {
		t.Fatal("expected luxview checkout to be called")
	}
	if legacy.called {
		t.Fatal("did not expect legacy checkout to be called")
	}
	if result.Ref != "luxview" {
		t.Fatalf("Ref = %q, want luxview", result.Ref)
	}
}

func TestAppSourceCheckoutUsesLegacyWhenRepositoryIDIsMissing(t *testing.T) {
	legacy := &fakeSourceCheckout{name: "legacy"}
	luxview := &fakeSourceCheckout{name: "luxview"}
	checkout := NewAppSourceCheckout(legacy, luxview)

	result, err := checkout.Checkout(context.Background(), &model.App{}, "main", t.TempDir())
	if err != nil {
		t.Fatalf("Checkout() error = %v", err)
	}
	if !legacy.called {
		t.Fatal("expected legacy checkout to be called")
	}
	if luxview.called {
		t.Fatal("did not expect luxview checkout to be called")
	}
	if result.Ref != "legacy" {
		t.Fatalf("Ref = %q, want legacy", result.Ref)
	}
}
