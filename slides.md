---
theme: seriph
title: hclbuilder — Make HCL changes explicit
info: |
  An introduction to hclbuilder, a Go library for incrementally modifying
  and formatting HashiCorp Configuration Language (HCL).
author: magodo
class: text-center
colorSchema: dark
drawings:
  persist: false
transition: fade-out
duration: 20min
---

<div class="mt-22">

# `hclbuilder`

<div class="text-2xl mt-5 opacity-90">
Make HCL changes explicit
</div>

<div class="mt-10 text-lg opacity-65">
A Go library for incrementally modifying and formatting HCL
</div>

</div>

<div class="abs-br m-8 text-sm opacity-55">
github.com/magodo/hclbuilder
</div>

<!--
Open with the central idea: tests should say what changed, not repeat everything
that stayed the same.
-->

---
layout: statement
---

# Configuration is usually read as a whole.

<div v-click class="mt-10 text-3xl opacity-80">
Tests often need to express it as a <span class="text-amber-300">sequence of changes</span>.
</div>

<!--
HCL is declarative, but a multi-step acceptance test has a timeline. That
mismatch is the problem hclbuilder addresses.
-->

---
layout: two-cols
layoutClass: gap-12
---

# The acceptance-test problem

Terraform provider acceptance tests require every `TestStep` to receive a **complete configuration string**.

But adjacent steps may change only one or two fields.

<div v-click class="mt-8 p-4 rounded bg-red-500:10 border border-red-400:25">

**The test's intent gets buried:**

- duplicated HCL drifts apart
- helper flags create hidden branches
- reviewers compare large strings by eye

</div>

::right::

<div class="mt-14">

```go
Steps: []resource.TestStep{
  {Config: configInitial()},
  {Config: configUpdated()},
  {Config: configWithoutTags()},
}
```

<div v-click class="text-center my-5 opacity-55">or</div>

```go
Steps: []resource.TestStep{
  {Config: config(false, false)},
  {Config: config(true, false)},
  {Config: config(true, true)},
}
```

<div v-click class="mt-5 text-center text-amber-300">
What did each step actually change?
</div>

</div>

<!--
Two common approaches: duplicate configuration functions, or one highly
parameterized function. Both force the reader to reconstruct the diff.
-->

---
layout: center
class: text-center
---

# Start once. Clone. Describe the delta.

<div class="flex items-center justify-center gap-5 mt-14 text-xl">
  <div class="px-6 py-4 rounded-xl bg-sky-500:15 border border-sky-400:30">
    base HCL
  </div>
  <div class="text-3xl opacity-45">→</div>
  <div v-click class="px-6 py-4 rounded-xl bg-violet-500:15 border border-violet-400:30">
    `Clone()`
  </div>
  <div v-click class="text-3xl opacity-45">→</div>
  <div v-click class="px-6 py-4 rounded-xl bg-amber-500:15 border border-amber-400:30">
    targeted edits
  </div>
  <div v-click class="text-3xl opacity-45">→</div>
  <div v-click class="px-6 py-4 rounded-xl bg-emerald-500:15 border border-emerald-400:30">
    formatted HCL
  </div>
</div>

<div v-click class="mt-14 text-2xl opacity-80">
The source code becomes a readable history of the configuration.
</div>

---
layout: two-cols
layoutClass: gap-8
---

# A step reads like a diff

```hcl
resource "example_widget" "test" {
  name = "initial"

  settings = {
    retries = 3
  }

  tags = {
    environment = "test"
  }
}
```

::right::

<div class="mt-12">

```go {all|1-3|5-6|8-9}
base := hclbuilder.New(source)

updated := base.Clone().
  SetExpressionAt(
    `[resource.example_widget.test].name`,
    `"renamed"`).
  SetExpressionAt(
    `[resource.example_widget.test].settings.retries`,
    `5`)

withoutTags := updated.Clone().
  RemoveAt(
    `[resource.example_widget.test].tags`)
```

</div>

<div v-click class="mt-5 p-3 rounded bg-emerald-500:10 border border-emerald-400:25 text-sm">
Every builder still produces the complete config expected by the test framework.
</div>

<!--
Walk through the clicks: construct the base, clone and update two expressions,
then branch again and remove tags. The original builders remain available.
-->

---
layout: two-cols
layoutClass: gap-10
---

# Use every version directly

```go
resource.Test(t, resource.TestCase{
  // ...
  Steps: []resource.TestStep{
    {Config: base.BuildString()},
    {Config: updated.BuildString()},
    {Config: withoutTags.BuildString()},
  },
})
```

<div v-click class="mt-8 text-sm opacity-65">
`Build()` returns formatted `[]byte`.<br>
`BuildString()` returns the same result as a `string`.
</div>

::right::

<div class="mt-12">

| Builder | Intent |
|---|---|
| `base` | create the widget |
| `updated` | rename it; change retries |
| `withoutTags` | remove tags |

</div>

<div v-click class="mt-10 text-xl text-emerald-300">
No string diff required to understand the scenario.
</div>

---

# Addresses follow the HCL structure

<div class="mt-8 grid grid-cols-[1.05fr_1fr] gap-10">

<div>

```hcl
resource "server" "web" {        # [resource.server.web]
  size = "small"                 # .size

  metadata = {                   # .metadata
    owner = "team-a"             # .owner
  }

  disk {                         # .[disk]
    mount = "/data"              # .mount
  }
}
```

</div>

<div class="space-y-4 mt-2">

<div v-click class="p-3 rounded bg-sky-500:10 border border-sky-400:25">
<div class="font-mono text-sm text-sky-300">[resource.server.web].size</div>
<div class="text-sm opacity-65 mt-1">an attribute in a labeled block</div>
</div>

<div v-click class="p-3 rounded bg-violet-500:10 border border-violet-400:25">
<div class="font-mono text-sm text-violet-300">[resource.server.web].metadata.owner</div>
<div class="text-sm opacity-65 mt-1">an item in a nested object</div>
</div>

<div v-click class="p-3 rounded bg-amber-500:10 border border-amber-400:25">
<div class="font-mono text-sm text-amber-300">[resource.server.web].[disk].mount</div>
<div class="text-sm opacity-65 mt-1">an attribute in a nested block</div>
</div>

</div>
</div>

<!--
Square brackets distinguish block steps from attributes and object keys.
Block steps must come first because HCL bodies contain blocks, while expressions
contain objects and tuples.
-->

---
layout: two-cols
layoutClass: gap-12
---

# Address grammar

```text
[block-type(.label)*(.index)?].key.index
```

<div class="mt-8 space-y-4 text-sm">

<div>
  <code>[service.api]</code>
  <span class="opacity-60 ml-2">first matching block</span>
</div>
<div>
  <code>[service.api.1]</code>
  <span class="opacity-60 ml-2">second matching block</span>
</div>
<div>
  <code>[service."1"]</code>
  <span class="opacity-60 ml-2">numeric label, not index</span>
</div>
<div>
  <code>objects.0.name</code>
  <span class="opacity-60 ml-2">tuple traversal</span>
</div>

</div>

::right::

<div class="mt-10 p-5 rounded-xl bg-white:5 border border-white:10">

### Reading an address

```text
[resource.aws_instance.web]
.tags
.Name
```

<div class="mt-5 grid grid-cols-[auto_1fr] gap-x-4 gap-y-3 text-sm">
  <span class="text-sky-300">block</span><span class="opacity-65">type + labels</span>
  <span class="text-violet-300">key</span><span class="opacity-65">`tags` attribute</span>
  <span class="text-amber-300">key</span><span class="opacity-65">`Name` object item</span>
</div>

</div>

<div v-click class="mt-6 text-sm opacity-60">
An omitted block index defaults to `0`.
</div>

---
layout: two-cols
layoutClass: gap-10
---

# API style 1 — operate at an address

Use concise, chainable operations when the edit is easy to name.

```go
builder.
  SetExpressionAt(
    `[resource.server.test].size`,
    `"large"`).
  SetExpressionAt(
    `[resource.server.test].metadata.region`,
    `"west"`).
  AppendBlockAt(
    `[resource.server.test]`,
    `lifecycle { prevent_destroy = true }`).
  RemoveAt(
    `[resource.server.test].legacy`)
```

::right::

<div class="mt-16 space-y-5">

<div class="p-4 rounded border border-sky-400:25 bg-sky-500:10">
  <code>SetExpressionAt</code>
  <div class="text-sm opacity-65 mt-1">set an attribute or object item</div>
</div>

<div class="p-4 rounded border border-violet-400:25 bg-violet-500:10">
  <code>AppendBlockAt</code>
  <div class="text-sm opacity-65 mt-1">append to a file or block body</div>
</div>

<div class="p-4 rounded border border-amber-400:25 bg-amber-500:10">
  <code>RemoveAt</code>
  <div class="text-sm opacity-65 mt-1">remove an attribute, object item, or block</div>
</div>

</div>

---
layout: two-cols
layoutClass: gap-10
---

# API style 2 — select, then modify

`At` resolves a node and passes its typed builder to a callback.

```go
builder.At(
  `[resource.server.test].metadata`,
  func(node hclbuilder.Builder) {
    node.AsObject().
      SetItem("owner", `"team-b"`).
      SetItem("region", `"west"`).
      RemoveItem("legacy")
  },
)
```

<div v-click class="mt-5 text-sm opacity-65">
Useful for several edits to one node, or for node-specific operations.
</div>

::right::

<div class="mt-12">

```text
Builder
  ├── AsFile()   → *FileBuilder
  ├── AsBlock()  → *BlockBuilder
  ├── AsObject() → *ObjectBuilder
  └── AsTuple()  → *TupleBuilder
```

<div v-click class="mt-10 p-4 rounded bg-emerald-500:10 border border-emerald-400:25">
The direct and callback styles are complementary—and can be mixed on one builder.
</div>

</div>

---
layout: center
---

# Not string replacement

<div class="flex items-center justify-center gap-4 mt-12 text-lg">
  <div class="px-5 py-4 rounded-xl bg-sky-500:12 border border-sky-400:25 text-center">
    <div class="font-mono">HCL source</div>
    <div class="text-xs opacity-55 mt-1">`[]byte`</div>
  </div>
  <div class="opacity-40 text-2xl">→</div>
  <div v-click class="px-5 py-4 rounded-xl bg-violet-500:12 border border-violet-400:25 text-center">
    <div>parse</div>
    <div class="text-xs opacity-55 mt-1">`hclwrite`</div>
  </div>
  <div v-click class="opacity-40 text-2xl">→</div>
  <div v-click class="px-5 py-4 rounded-xl bg-amber-500:12 border border-amber-400:25 text-center">
    <div>resolve address</div>
    <div class="text-xs opacity-55 mt-1">file / block / object / tuple</div>
  </div>
  <div v-click class="opacity-40 text-2xl">→</div>
  <div v-click class="px-5 py-4 rounded-xl bg-pink-500:12 border border-pink-400:25 text-center">
    <div>mutate node</div>
    <div class="text-xs opacity-55 mt-1">parsed expressions & blocks</div>
  </div>
  <div v-click class="opacity-40 text-2xl">→</div>
  <div v-click class="px-5 py-4 rounded-xl bg-emerald-500:12 border border-emerald-400:25 text-center">
    <div>format</div>
    <div class="text-xs opacity-55 mt-1">complete HCL</div>
  </div>
</div>

<div v-click class="mt-12 text-center text-xl opacity-80">
Edits target syntax nodes; output is normalized by the HCL formatter.
</div>

<!--
Expressions supplied to setters are parsed as HCL expressions. Blocks supplied
to append operations are parsed and validated as exactly one block.
-->

---
layout: two-cols
layoutClass: gap-14
---

# Try it

```bash
# Add the library
go get github.com/magodo/hclbuilder
```

<div class="mt-7 text-sm opacity-65">
Until the relevant HCL changes are merged:
</div>

```bash
go mod edit \
  -replace=github.com/hashicorp/hcl/v2=\
github.com/magodo/hcl/v2@dev

go mod tidy
```

::right::

<div class="mt-8">

```go
package main

import "github.com/magodo/hclbuilder"

func main() {
  result := hclbuilder.New(
    []byte(`name = "before"`),
  ).
    SetExpressionAt("name", `"after"`).
    BuildString()

  println(result)
}
```

<div v-click class="mt-5 text-center font-mono text-emerald-300">
name = "after"
</div>

</div>

---
layout: center
class: text-center
---

# Make each test step say what changed.

<div class="mt-10 text-2xl opacity-75">
Parse once · clone freely · edit structurally · build complete HCL
</div>

<div class="mt-14">
  <a href="https://github.com/magodo/hclbuilder" class="text-xl text-sky-300">
    github.com/magodo/hclbuilder
  </a>
</div>

<div class="mt-10 text-lg opacity-55">
Questions?
</div>
