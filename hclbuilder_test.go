package hclbuilder_test

import (
	"testing"

	hclbuilder "github.com/LingyuTang/hcl-builder"
	"github.com/stretchr/testify/require"
)

func TestFileBuilder_Clone(t *testing.T) {
	b := hclbuilder.New()
	require.Equal(t, []byte("a = 1\nb = 2\n"), b.SetAttribute("a", []byte("1")).Clone().Clone().SetAttribute("b", []byte("2")).Build())
	require.Equal(t, []byte("a = 1\n"), b.Build())
}
