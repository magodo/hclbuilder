// Package hclbuilder provides builders for modifying HashiCorp Configuration
// Language (HCL) files.
//
// Nodes are addressed with dot-separated paths. An address uses this syntax:
//
//	[block-type(.block-label)*(.block-index)?].key.index
//
// Block steps must come before key and tuple-index steps. An omitted block
// index defaults to 0. A final numeric block label must be quoted; otherwise,
// it is interpreted as a block index.
// For example, [resource.aws_instance.web].tags.Name addresses the Name item
// in the tags object of the web resource block.
package hclbuilder
