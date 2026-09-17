package hclbuilder_test

import (
	"fmt"

	"github.com/magodo/hclbuilder"
)

func Example_buildHCL() {
	b := hclbuilder.New(
		[]byte(`
string = "foo"
object = {
  bar = 5
  baz = true
  foo = "foo"
}

foo {
  hello = "world"
}

empty a {
}
empty a {
  x = 1
}
empty2 {
}
empty2 {
}

bar "a" "b" {
  baz {
	  object = {
		  nest = {
			  a = 1
			  x = 1
		  }
	  }
  }
}
`),
	)
	b.RenameAttribute("string", "foo").
		RemoveAttribute("object").
		At("[foo]", func(b hclbuilder.NodeBuilder) {
			b.AsBlock().
				RemoveAttribute("hello").
				SetAttribute("q", []byte("1")).
				SetAttribute("p", []byte(`"abc"`)).
				AppendNewBlock("bar", []string{"baz", "0"}, func(b *hclbuilder.BlockBuilder) {
					b.
						SetAttribute("z", []byte("1")).
						AppendBlock([]byte(`x y {
							foo = [1, 2, 3]
						}
						`))
				})
		}).
		RemoveBlocks("empty", []string{"a"}, []int{0}).
		RemoveBlocks("empty2", nil, nil).
		At("[bar.a.b].[baz].object.nest", func(b hclbuilder.NodeBuilder) {
			b.AsObject().
				SetItem("a", []byte("2")).
				SetItem("b", []byte(`"hello"`)).
				RemoveItem("x")
		}).
		SetAttribute("bar", []byte(`"bar"`))
	fmt.Println(string(b.Build()))
	// Output:
	// foo = "foo"
	//
	// foo {
	//   q = 1
	//   p = "abc"
	//   bar "baz" "0" {
	//     z = 1
	//     x y {
	//       foo = [1, 2, 3]
	//     }
	//   }
	// }
	//
	// empty a {
	//   x = 1
	// }
	//
	// bar "a" "b" {
	//   baz {
	//     object = {
	//       nest = {
	//         a = 2
	//         b = "hello"
	//       }
	//     }
	//   }
	// }
	// bar = "bar"
}
