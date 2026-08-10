package embed

import (
	"context"
	"errors"
	"testing"
)

func TestNewReturnsKeywordOnlyNoop(t *testing.T) {
	e := New()
	if e == nil {
		t.Fatal("New returned nil Embedder")
	}
	if got := e.Dimensions(); got != Dimensions {
		t.Errorf("Dimensions() = %d, want %d", got, Dimensions)
	}

	vecs, err := e.Embed(context.Background(), []string{"anything"})
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("Embed err = %v, want ErrUnavailable", err)
	}
	if vecs != nil {
		t.Errorf("Embed vecs = %v, want nil", vecs)
	}
}

func TestNoopMatchesNew(t *testing.T) {
	if _, err := Noop().Embed(context.Background(), nil); !errors.Is(err, ErrUnavailable) {
		t.Errorf("Noop().Embed err = %v, want ErrUnavailable", err)
	}
}
