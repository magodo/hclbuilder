package hclbuilder

type Option func(*Builder)

// WithErrorFunc sets a custom error function.
func WithErrorFunc(ef ErrorFunc) Option {
	return func(b *Builder) {
		b.ef = ef
	}
}
