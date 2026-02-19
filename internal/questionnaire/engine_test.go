package questionnaire

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

type fakeRenderer struct {
	responses map[string][]string
	asked     map[string]int
	err       error
}

func (f *fakeRenderer) Ask(_ context.Context, _ FormSpec, field FieldSpec, _ string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	if f.asked == nil {
		f.asked = map[string]int{}
	}
	f.asked[field.Name]++
	vals := f.responses[field.Name]
	if len(vals) == 0 {
		return "", nil
	}
	out := vals[0]
	f.responses[field.Name] = vals[1:]
	return out, nil
}

func TestCollectRetriesUntilValid(t *testing.T) {
	r := &fakeRenderer{responses: map[string][]string{"at": {"", "2030-01-01 10:00"}}}
	e := Engine{Renderer: r}

	_, err := e.Collect(context.Background(), FormSpec{Fields: []FieldSpec{{
		Name:     "at",
		Required: true,
		Validate: func(value string) error {
			if value != "2030-01-01 10:00" {
				return fmt.Errorf("invalid")
			}
			return nil
		},
	}}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.asked["at"] != 2 {
		t.Fatalf("expected 2 prompts, got %d", r.asked["at"])
	}
}

func TestCollectSkipsValidPreset(t *testing.T) {
	r := &fakeRenderer{responses: map[string][]string{"id": {"should-not-be-used"}}}
	e := Engine{Renderer: r}

	ans, err := e.Collect(context.Background(), FormSpec{Fields: []FieldSpec{{Name: "id", Required: true}}}, Answers{"id": "a_123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ans["id"] != "a_123" {
		t.Fatalf("unexpected answer: %s", ans["id"])
	}
	if r.asked["id"] != 0 {
		t.Fatalf("expected no prompt, got %d", r.asked["id"])
	}
}

func TestCollectCancelError(t *testing.T) {
	r := &fakeRenderer{err: ErrCanceled}
	e := Engine{Renderer: r}

	_, err := e.Collect(context.Background(), FormSpec{Fields: []FieldSpec{{Name: "id", Required: true}}}, nil)
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("expected ErrCanceled, got %v", err)
	}
}
