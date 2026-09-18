package hclbuilder_test

import (
	"testing"

	"github.com/magodo/hclbuilder"
	"github.com/stretchr/testify/require"
)

// requireHCLEqual normalizes expected through the same parse+format pipeline as the
// actual value, so leading-whitespace differences (tabs vs spaces) in the raw
// fixture don't affect the comparison.
func requireHCLEqual(t *testing.T, expected, actual string) {
	t.Helper()
	require.Equal(t, hclbuilder.New([]byte(expected)).BuildString(), actual)
}

func TestSetAt(t *testing.T) {
	const template = `
root = "old"

type_a {
  a = "old"
  object = {
    key = "old"
  }
}

type_b "boo" "1" {
  sub {
    value = "first"
  }
  sub {
    value = "second"
  }
}
`

	t.Run("set attributes and object", func(t *testing.T) {
		b := hclbuilder.New([]byte(template))
		b.SetAt("root", `"new root"`)
		b.SetAt("[type_a].a", `"new"`)
		b.SetAt("[type_a].object.key", `"new object value"`)
		b.SetAt(`[type_b.boo."1"].[sub.1].value`, `"new nested value"`)

		// Set values that don't exist yet.
		b.SetAt("added_root", `"added root"`)
		b.SetAt("[type_a].added_value", `"added value"`)
		b.SetAt("[type_a].object.added_key", `"added object value"`)
		b.SetAt("[type_a].added_object", `{
  key   = "whole object value"
  extra = true
}`)

		expected := `
root = "new root"

type_a {
  a = "new"
  object = {
    key       = "new object value"
    added_key = "added object value"
  }
  added_value = "added value"
  added_object = {
    key   = "whole object value"
    extra = true
  }
}

type_b "boo" "1" {
  sub {
    value = "first"
  }
  sub {
    value = "new nested value"
  }
}
added_root = "added root"
`
		requireHCLEqual(t, expected, b.BuildString())
	})

	t.Run("final step is a block", func(t *testing.T) {
		var gotErr error
		b := hclbuilder.New([]byte(template), hclbuilder.WithErrorFunc(func(err error) { gotErr = err }))
		before := b.BuildString()
		b.SetAt("[type_a].[object]", `"x"`)
		require.ErrorContains(t, gotErr, "not pointing to an attribute or object")
		require.Equal(t, before, b.BuildString())
	})

	t.Run("parent is not a block or object", func(t *testing.T) {
		var gotErr error
		b := hclbuilder.New([]byte(`list = [1, 2]`), hclbuilder.WithErrorFunc(func(err error) { gotErr = err }))
		before := b.BuildString()
		b.SetAt("list.key", `"x"`)
		require.ErrorContains(t, gotErr, "parent is not a block or object body")
		require.Equal(t, before, b.BuildString())
	})

	t.Run("invalid address", func(t *testing.T) {
		var gotErr error
		b := hclbuilder.New([]byte(template), hclbuilder.WithErrorFunc(func(err error) { gotErr = err }))
		before := b.BuildString()
		b.SetAt("[nonexistent].key", `"x"`)
		require.ErrorContains(t, gotErr, "node not found")
		require.Equal(t, before, b.BuildString())
	})
}

func TestAppendBlockAt(t *testing.T) {
	const template = `
root {
  child {
    value = "old"
  }
}
`
	t.Run("append blocks", func(t *testing.T) {
		b := hclbuilder.New([]byte(template))
		b.AppendBlockAt("", `sibling {}`)
		b.AppendBlockAt("[root]", `added {
  value = "added"
}`)
		b.AppendBlockAt("[root].[child]", `grandchild {
      object = {
        a = 1
      }
    }`)

		expected := `
root {
  child {
    value = "old"
    grandchild {
      object = {
        a = 1
      }
    }
  }
  added {
    value = "added"
  }
}
sibling {}
`
		requireHCLEqual(t, expected, b.BuildString())
	})

	t.Run("not a file or block body", func(t *testing.T) {
		var gotErr error
		b := hclbuilder.New([]byte(`list = [1, 2]`), hclbuilder.WithErrorFunc(func(err error) { gotErr = err }))
		before := b.BuildString()
		b.AppendBlockAt("list", `child {}`)
		require.ErrorContains(t, gotErr, "not a file or block body")
		require.Equal(t, before, b.BuildString())
	})

	t.Run("invalid address", func(t *testing.T) {
		var gotErr error
		b := hclbuilder.New([]byte(template), hclbuilder.WithErrorFunc(func(err error) { gotErr = err }))
		before := b.BuildString()
		b.AppendBlockAt("[nonexistent]", `child {}`)
		require.ErrorContains(t, gotErr, "node not found")
		require.Equal(t, before, b.BuildString())
	})
}

func TestRemoveAt(t *testing.T) {
	const template = `
type_a {
  a    = "old"
  list = [1, 2]
}

type_b "label1"{
  name = "test"
  object = {
    owner       = "foo"
  }
  object2 = {
    owner       = "foo"
  }
  nested {
    value = "old"
  }
  sub {
    name = "first"
  }
  sub {
    name = "second"
  }
}
`

	t.Run("remove nodes", func(t *testing.T) {
		expected := `

type_b "label1" {
  object = {
  }
  nested {
  }
  sub {
    name = "first"
  }
}
`
		b := hclbuilder.New([]byte(template))
		b.RemoveAt("[type_a]")
		b.RemoveAt("[type_b.label1].name")
		b.RemoveAt("[type_b.label1].object.owner")
		b.RemoveAt("[type_b.label1].object2")
		b.RemoveAt("[type_b.label1].[nested].value")
		b.RemoveAt("[type_b.label1].[sub.1]")
		requireHCLEqual(t, expected, b.BuildString())
	})

	t.Run("unsupported final step", func(t *testing.T) {
		var gotErr error
		b := hclbuilder.New([]byte(template), hclbuilder.WithErrorFunc(func(err error) { gotErr = err }))
		before := b.BuildString()
		b.RemoveAt("[type_a].list.0")
		require.ErrorContains(t, gotErr, "unsupported final step")
		require.Equal(t, before, b.BuildString())
	})

	t.Run("remove attribute with non-body parent", func(t *testing.T) {
		var gotErr error
		b := hclbuilder.New([]byte(template), hclbuilder.WithErrorFunc(func(err error) { gotErr = err }))
		before := b.BuildString()
		b.RemoveAt("[type_a].list.key")
		require.ErrorContains(t, gotErr, "parent is not a file, block or object")
		require.Equal(t, before, b.BuildString())
	})

	t.Run("invalid address", func(t *testing.T) {
		var gotErr error
		b := hclbuilder.New([]byte(template), hclbuilder.WithErrorFunc(func(err error) { gotErr = err }))
		before := b.BuildString()
		b.RemoveAt("[nonexistent].name")
		require.ErrorContains(t, gotErr, "node not found")
		require.Equal(t, before, b.BuildString())
	})
}
