package auto

import (
	"context"
	"errors"
	"testing"

	"agenda-pacientes/internal/questionnaire"
	"agenda-pacientes/internal/questionnaire/ui/webview"
)

type stubRenderer struct {
	value string
	err   error
}

func (s stubRenderer) Ask(_ context.Context, _ questionnaire.FormSpec, _ questionnaire.FieldSpec, _ string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.value, nil
}

func TestAutoFallsBackToTTYOnWebUnavailable(t *testing.T) {
	r := Renderer{
		Web: stubRenderer{err: webview.ErrUnavailable},
		TTY: stubRenderer{value: "ok"},
	}
	value, err := r.Ask(context.Background(), questionnaire.FormSpec{}, questionnaire.FieldSpec{Name: "x"}, "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if value != "ok" {
		t.Fatalf("unexpected value: %s", value)
	}
}

func TestAutoReturnsWebErrorWhenUnexpected(t *testing.T) {
	boom := errors.New("boom")
	r := Renderer{Web: stubRenderer{err: boom}, TTY: stubRenderer{value: "ok"}}
	_, err := r.Ask(context.Background(), questionnaire.FormSpec{}, questionnaire.FieldSpec{Name: "x"}, "")
	if !errors.Is(err, boom) {
		t.Fatalf("expected boom, got %v", err)
	}
}
