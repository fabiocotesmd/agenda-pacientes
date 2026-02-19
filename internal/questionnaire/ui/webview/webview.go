package webview

import (
	"context"
	"errors"
	"os"

	"agenda-pacientes/internal/questionnaire"
)

var ErrUnavailable = errors.New("webview unavailable")

type Renderer struct{}

func (Renderer) IsAvailable() bool {
	return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != "" || os.Getenv("QUESTIONNAIRE_WEBVIEW") == "1"
}

func (r Renderer) Ask(_ context.Context, _ questionnaire.FormSpec, _ questionnaire.FieldSpec, _ string) (string, error) {
	if !r.IsAvailable() {
		return "", ErrUnavailable
	}
	// Phase 1: WebView adapter placeholder to keep the interface stable.
	// Auto mode falls back to TTY when this adapter is unavailable.
	return "", ErrUnavailable
}
