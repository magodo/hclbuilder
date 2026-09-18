package hclbuilder_test

import (
	"testing"

	"github.com/magodo/hclbuilder"
	"github.com/stretchr/testify/require"
)

func TestBuilder_FromEmptyFile(t *testing.T) {
	b := hclbuilder.New(nil).
		AppendBlock("blk1 {}").
		AppendBlock(`blk2 {
			foo = 1
		}`).
		SetAttribute("x", "1").
		SetAttribute("obj", `{ y = "y"}`)
	expect := `blk1 {}
blk2 {
  foo = 1
}
x   = 1
obj = { y = "y" }
`
	require.Equal(t, b.BuildString(), expect)
}

func TestBuilder_Clone(t *testing.T) {
	b := hclbuilder.New(nil)
	require.Equal(t, "a = 1\nb = 2\n", b.SetAttribute("a", "1").Clone().SetAttribute("b", "2").BuildString())
	require.Equal(t, "a = 1\n", b.BuildString())
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
		require.Equal(t, "a = 1",
			hclbuilder.New([]byte("a = 1"), hclbuilder.WithErrorFunc(matchErr(t, "node not found"))).At("b", nil).BuildString())
	})
	t.Run("block builder failed", func(t *testing.T) {
		require.Equal(t, "foo {\n}\n",
			hclbuilder.New(nil, hclbuilder.WithErrorFunc(matchErr(t, "node not found"))).AppendNewBlock("foo", nil, func(bb *hclbuilder.BlockBuilder) {
				bb.At("x", nil)
			}).BuildString())
	})
}
