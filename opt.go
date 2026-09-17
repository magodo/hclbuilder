package hclbuilder

type Option func(*Builder)

func WithErrorFunc(ef ErrorFunc) Option {
	return func(b *Builder) {
		b.ef = ef
	}
}
