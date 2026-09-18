# hclbuilder

`hclbuilder` is a Go library for incrementally modifying and formatting HashiCorp Configuration Language (HCL).

## Motivation

Terraform provider acceptance tests require every `TestStep` to provide its complete Terraform configuration as a string. A test often contains several steps whose configurations differ in only a few places.

This commonly leads to one of two patterns:

- A separate function for every almost-identical configuration
- One configuration function with several parameters or flags that subtly affect the generated HCL.

In both cases, it becomes difficult to see what a test step actually changes.

With `hclbuilder`, a test can define one basic configuration, conditionally clone it for some step, and describe each change explicitly. Every step still receives the complete string expected by the acceptance-test framework, but the test source focuses on the differences between steps.

```go
base := hclbuilder.New([]byte(`
resource "example_widget" "test" {
  name = "initial"

  settings = {
    retries = 3
  }

  tags = {
    environment = "test"
  }
}
`))

updated := base.Clone().
    SetExpressionAt(`[resource.example_widget.test].name`, `"renamed"`).
    SetExpressionAt(`[resource.example_widget.test].settings.retries`, `5`)

withoutTags := updated.Clone().
    RemoveAt(`[resource.example_widget.test].tags`)

resource.Test(t, resource.TestCase{
    // ...
    Steps: []resource.TestStep{
        {Config: base.BuildString()},
        {Config: updated.BuildString()},
        {Config: withoutTags.BuildString()},
    },
})
```

The second step clearly renames the widget and changes its retry count. The third clearly removes `tags`. There is no need to compare large string literals or work out what a collection of boolean parameters does.

## Installation

```sh
go get github.com/magodo/hclbuilder

```

The below steps are not needed until the relevant PRs in `hcl` are merged.

```she
go mod edit -replace=github.com/hashicorp/hcl/v2=github.com/magodo/hcl/v2@dev
go mod tidy
```

## Basic usage

Start with an existing HCL document, make one or more changes, and build the formatted result. There are two complementary API styles:

1. Use the `XXXAt()` method of the `FileBuilder` to modify the node at an address directly.
2. Use the `At` to select a node, then modify it through the builder passed to the callback.

The two styles can be mixed on the same builder.

### Modify an address directly with `XXXAt`

Methods such as `SetExpressionAt`, `AppendBlockAt`, and `RemoveAt` are convenient when the desired operation can be expressed directly:

```go
builder := hclbuilder.New([]byte(`
resource "example_server" "test" {
  size = "small"
  metadata = {
    owner = "team-a"
  }
}
`))

builder.
    SetExpressionAt(`[resource.example_server.test].size`, `"large"`).
    SetExpressionAt(`[resource.example_server.test].metadata.owner`, `"team-b"`).
    SetExpressionAt(`[resource.example_server.test].metadata.region`, `"west"`)
```

### Select an address with `At`

`At` supplies the builder corresponding to the addressed node. This style is useful when making multiple changes to one node or when using operations specific to a file, block, object, or tuple. Use `AsFile`, `AsBlock`, `AsObject`, or `AsTuple` to select its concrete kind:

```go
builder.At(`[resource.example_server.test].metadata`, func(node hclbuilder.Builder) {
    node.AsObject().
        SetItem("owner", `"team-b"`).
        SetItem("region", `"west"`)
})
```
