package questionnaire

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var ErrCanceled = errors.New("questionnaire canceled")

type Engine struct {
	Renderer Renderer
}

func (e Engine) Collect(ctx context.Context, form FormSpec, preset Answers) (Answers, error) {
	if e.Renderer == nil {
		return nil, fmt.Errorf("questionnaire renderer is required")
	}

	answers := Answers{}
	for k, v := range preset {
		answers[k] = v
	}

	for _, field := range form.Fields {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if existing, ok := answers[field.Name]; ok {
			if err := validateField(field, existing); err == nil {
				continue
			}
		}

		for {
			value, err := e.Renderer.Ask(ctx, form, field, answers[field.Name])
			if err != nil {
				return nil, err
			}
			if err := validateField(field, value); err != nil {
				continue
			}
			answers[field.Name] = strings.TrimSpace(value)
			break
		}
	}

	return answers, nil
}

func validateField(field FieldSpec, value string) error {
	trimmed := strings.TrimSpace(value)
	if field.Required && trimmed == "" {
		return fmt.Errorf("%s es obligatorio", field.Name)
	}
	if trimmed == "" {
		return nil
	}
	if field.Validate != nil {
		if err := field.Validate(trimmed); err != nil {
			return err
		}
	}
	return nil
}
