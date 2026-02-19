package tty

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"agenda-pacientes/internal/questionnaire"
)

type Renderer struct {
	In  io.Reader
	Out io.Writer
	br  *bufio.Reader
}

func (r *Renderer) Ask(_ context.Context, _ questionnaire.FormSpec, field questionnaire.FieldSpec, current string) (string, error) {
	in := r.In
	out := r.Out
	if in == nil || out == nil {
		return "", fmt.Errorf("tty renderer requires input and output streams")
	}
	if r.br == nil {
		r.br = bufio.NewReader(in)
	}

	label := field.Label
	if strings.TrimSpace(label) == "" {
		label = field.Name
	}

	defaultValue := strings.TrimSpace(current)
	if defaultValue == "" {
		defaultValue = strings.TrimSpace(field.Default)
	}

	if defaultValue != "" {
		fmt.Fprintf(out, "%s [%s]: ", label, defaultValue)
	} else {
		fmt.Fprintf(out, "%s: ", label)
	}

	if strings.TrimSpace(field.Help) != "" {
		fmt.Fprintf(out, "\n  %s\n", field.Help)
	}

	line, err := r.br.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}

	trimmed := strings.TrimSpace(line)
	if strings.EqualFold(trimmed, "cancel") || strings.EqualFold(trimmed, ":q") {
		return "", questionnaire.ErrCanceled
	}
	if trimmed == "" {
		return defaultValue, nil
	}
	return trimmed, nil
}
