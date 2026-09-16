package hclbuilder_test

import (
	"fmt"

	hclbuilder "github.com/LingyuTang/hcl-builder"
)

func Example_buildHCL() {
	b := hclbuilder.New()

	b.SetContent([]byte(`
string = "foo"
object = {
  bar = 5
  baz = true
  foo = "foo"
}

foo {
  hello = "world"
}
empty {
}

bar "a" "b" {
  baz {
	  object = {
		  nest = {
			  a = 1
		  }
	  }
  }
}
`)).
		RenameAttribute("string", "foo").
		RemoveAttribute("object").
		At("[foo]", func(b hclbuilder.Builder) {
			b.AsBlock().
				RemoveAttribute("hello").
				SetAttribute("q", []byte("1")).
				SetAttribute("p", []byte(`"abc"`)).
				AppendNewBlock("bar", []string{"baz"}, func(b *hclbuilder.BlockBuilder) {
					b.
						SetAttribute("z", []byte("1")).
						AppendBlock([]byte(`x y {
							foo = [1, 2, 3]
						}
						`))
				})
		}).
		RemoveBlocks("empty", nil, nil).
		At("[bar.a.b].[baz].object.nest", func(b hclbuilder.Builder) {
			b.AsObject().
				SetItem("a", []byte("2")).
				SetItem("b", []byte(`"hello"`))
		}).
		SetAttribute("bar", []byte(`"bar"`))
	fmt.Println(string(b.Build()))
	// Output:
	// foo = "foo"
	//
	// foo {
	//   q = 1
	//   p = "abc"
	//   bar "baz" {
	//     z = 1
	//     x y {
	//       foo = [1, 2, 3]
	//     }
	//   }
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
