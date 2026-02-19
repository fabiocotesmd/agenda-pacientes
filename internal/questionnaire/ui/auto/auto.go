package auto

import (
	"context"
	"errors"

	"agenda-pacientes/internal/questionnaire"
	"agenda-pacientes/internal/questionnaire/ui/tty"
	"agenda-pacientes/internal/questionnaire/ui/webview"
)

type Renderer struct {
	Web questionnaire.Renderer
	TTY questionnaire.Renderer
}

func New(ttyRenderer *tty.Renderer) Renderer {
	return Renderer{
		Web: webview.Renderer{},
		TTY: ttyRenderer,
	}
}

func (r Renderer) Ask(ctx context.Context, form questionnaire.FormSpec, field questionnaire.FieldSpec, current string) (string, error) {
	if r.Web != nil {
		value, err := r.Web.Ask(ctx, form, field, current)
		if err == nil {
			return value, nil
		}
		if !errors.Is(err, webview.ErrUnavailable) {
			return "", err
		}
	}
	if r.TTY == nil {
		return "", webview.ErrUnavailable
	}
	return r.TTY.Ask(ctx, form, field, current)
}
