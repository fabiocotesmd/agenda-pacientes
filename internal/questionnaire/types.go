package questionnaire

import "context"

// Answers stores resolved values by flag/field name.
type Answers map[string]string

type FieldType string

const (
	FieldTypeString   FieldType = "string"
	FieldTypeDateTime FieldType = "datetime"
	FieldTypeID       FieldType = "id"
)

type FieldSpec struct {
	Name     string
	Label    string
	Required bool
	Type     FieldType
	Default  string
	Help     string
	Validate func(value string) error
}

type FormSpec struct {
	CommandKey string
	Fields     []FieldSpec
}

type Renderer interface {
	Ask(ctx context.Context, form FormSpec, field FieldSpec, current string) (string, error)
}
