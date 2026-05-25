package services

import (
	"context"
	"strings"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
)

type MaskedPredefinedCodeRenderer struct{}

func (MaskedPredefinedCodeRenderer) RenderPredefinedCode(ctx context.Context, code store.PredefinedCode) (string, error) {
	return strings.TrimSpace(code.CodeMaskedDisplay), nil
}
