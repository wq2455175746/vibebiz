package scheduledtask

import (
	"context"
	"errors"
	"testing"

	"vibebiz/internal/sandboxregistry"
)

type fakeSelector struct {
	instance sandboxregistry.Instance
	err      error
}

func (f *fakeSelector) Select(context.Context) (sandboxregistry.Instance, error) {
	return f.instance, f.err
}

func TestSandboxParserRejectsEmptyPrompt(t *testing.T) {
	parser := NewSandboxParser(&fakeSelector{}, nil)
	if _, err := parser.Parse(context.Background(), "  "); !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v", err)
	}
}
