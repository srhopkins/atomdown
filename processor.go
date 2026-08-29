package atomdown

import (
	"context"
	"fmt"
)

// Extension adds application-specific behavior to the parsed document model.
// Extensions run in registration order and may decorate the document, preserve
// private state in attributes, or append extension-specific diagnostics.
type Extension interface {
	Name() string
	Transform(context.Context, []byte, *Document) error
}

// ExtensionFunc adapts a function into an Extension.
type ExtensionFunc struct {
	ExtensionName string
	TransformFunc func(context.Context, []byte, *Document) error
}

// Name returns the extension's stable name.
func (extension ExtensionFunc) Name() string { return extension.ExtensionName }

// Transform invokes the wrapped transform function.
func (extension ExtensionFunc) Transform(ctx context.Context, source []byte, document *Document) error {
	if extension.TransformFunc == nil {
		return nil
	}
	return extension.TransformFunc(ctx, source, document)
}

// Processor parses Atomdown and applies embedded extensions.
type Processor struct {
	extensions []Extension
}

// NewProcessor creates an embedded Atomdown processor.
func NewProcessor(extensions ...Extension) *Processor {
	return &Processor{extensions: append([]Extension(nil), extensions...)}
}

// Process parses source and runs each extension in registration order.
func (processor *Processor) Process(ctx context.Context, source []byte) (Document, error) {
	document := Parse(source)
	if processor == nil {
		return document, nil
	}
	for _, extension := range processor.extensions {
		if extension == nil {
			continue
		}
		if err := extension.Transform(ctx, source, &document); err != nil {
			return document, fmt.Errorf("Atomdown extension %q: %w", extension.Name(), err)
		}
	}
	return document, nil
}
