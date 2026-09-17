package hclbuilder_test

import (
	"testing"

	hclbuilder "github.com/LingyuTang/hcl-builder"
	"github.com/stretchr/testify/require"
)

func TestBuilder_Clone(t *testing.T) {
	b := hclbuilder.New(nil)
	require.Equal(t, []byte("a = 1\nb = 2\n"), b.SetAttribute("a", []byte("1")).Clone().Clone().SetAttribute("b", []byte("2")).Build())
	require.Equal(t, []byte("a = 1\n"), b.Build())
}

func TestBuilder_ErrorHandler(t *testing.T) {
	matchErr := func(t *testing.T, msg string) hclbuilder.ErrorFunc {
		return func(err error) {
			require.ErrorContains(t, err, msg)
		}
	}
	t.Run("new failed", func(t *testing.T) {
		hclbuilder.New([]byte("x=x=x"), hclbuilder.WithErrorFunc(matchErr(t, "Missing newline after argument")))
	})
	t.Run("at failed", func(t *testing.T) {
		hclbuilder.New(nil, hclbuilder.WithErrorFunc(matchErr(t, "node not found"))).At("a", nil)
	})
	t.Run("failed op is noop", func(t *testing.T) {
		require.Equal(t, []byte("a = 1"),
			hclbuilder.New([]byte("a = 1"), hclbuilder.WithErrorFunc(matchErr(t, "node not found"))).At("b", nil).Build())
	})
	t.Run("block builder failed", func(t *testing.T) {
		require.Equal(t, []byte("foo {\n}\n"),
			hclbuilder.New(nil, hclbuilder.WithErrorFunc(matchErr(t, "node not found"))).AppendNewBlock("foo", nil, func(bb *hclbuilder.BlockBuilder) {
				bb.At("x", nil)
			}).Build())
	})
}
