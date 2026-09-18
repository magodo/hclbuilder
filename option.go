package hclbuilder

// Option configures a FileBuilder.
type Option func(*FileBuilder)

// WithErrorFunc sets a custom error function.
func WithErrorFunc(ef ErrorFunc) Option {
	return func(b *FileBuilder) {
		b.ef = ef
	}
}
