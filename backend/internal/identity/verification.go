package identity

import (
	"context"
	"io"
)

type Result struct {
	State string
	Note  string
}

// Provider is the optional OCR/MRZ boundary. Implementations must never log
// document bytes or extracted identity fields. Hotels without an approved
// provider use ManualProvider and remain in the staff review queue.
type Provider interface {
	Verify(context.Context, io.Reader, string) (Result, error)
}

type ManualProvider struct{}

func (ManualProvider) Verify(_ context.Context, _ io.Reader, _ string) (Result, error) {
	return Result{State: "manual_review", Note: ""}, nil
}
