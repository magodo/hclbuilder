package hclbuilder_test

import (
	"testing"

	"github.com/magodo/hclbuilder"
	"github.com/stretchr/testify/require"
)

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

	t.Run("set attributes", func(t *testing.T) {
		b := hclbuilder.New([]byte(template))
		b.SetAt("root", `"new root"`)
		b.SetAt("[type_a].a", `"new block value"`)
		b.SetAt("[type_a].object.key", `"new object value"`)
		b.SetAt("[type_a].added_object", `{
			key   = "whole object value"
			extra = true
		}`)
		b.SetAt(`[type_b.boo."1"].[sub.1].value`, `"new nested value"`)

		// Set values that don't exist yet.
		b.SetAt("added_root", `"added root"`)
		b.SetAt("[type_a].added_block", `"added block value"`)
		b.SetAt("[type_a].object.added_key", `"added object value"`)

		expected := `
root = "new root"

type_a {
  a = "new block value"
  object = {
		key       = "new object value"
    added_key = "added object value"
  }
	added_object = {
		key   = "whole object value"
		extra = true
	}
  added_block = "added block value"
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
		require.Equal(t, expected, b.BuildString())
	})

	t.Run("final step is a block", func(t *testing.T) {
		var gotErr error
		b := hclbuilder.New([]byte(template), hclbuilder.WithErrorFunc(func(err error) { gotErr = err }))
		before := b.BuildString()
		b.SetAt("[type_a].[object]", `"x"`)
		require.ErrorContains(t, gotErr, "not pointing to an attribute or object")
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
		require.Equal(t, expected, b.BuildString())
	})

	t.Run("not a file or block body", func(t *testing.T) {
		var gotErr error
		b := hclbuilder.New([]byte(template), hclbuilder.WithErrorFunc(func(err error) { gotErr = err }))
		before := b.BuildString()
		b.AppendBlockAt("[root].[child].value", `child {}`)
		require.ErrorContains(t, gotErr, "not a file or block body")
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
    environment = "dev"
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
		require.Equal(t, expected, b.BuildString())
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
}
