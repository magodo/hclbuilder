package hclbuilder

import (
	"fmt"
	"slices"

	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

// Node is an interface implemented by all the hclwrite nodes.
type Node interface {
	BuildTokens(to hclwrite.Tokens) hclwrite.Tokens
}

type Builder interface {
	AsBlock() *BlockBuilder
	AsObject() *ObjectBuilder
	// TODO: Add AsTuple when TupleConsExpr is available
}

type FileBuilder struct {
	file *hclwrite.File
	body bodyOperator
}

func New() *FileBuilder {
	file := hclwrite.NewEmptyFile()
	return &FileBuilder{
		file: file,
		body: bodyOperator{body: file.Body()},
	}
}

// SetContent sets the HCL content to the builder. It overwrites any existing content.
func (b *FileBuilder) SetContent(src []byte) *FileBuilder {
	file, diags := hclwrite.ParseConfig(src, "", hcl.InitialPos)
	if diags.HasErrors() {
		panic(fmt.Sprintf("SetContent: %s", diags.Error()))
	}
	b.file = file
	b.body = bodyOperator{body: file.Body()}
	return b
}

// Build turns the HCL built so far into formatted bytes.
func (b FileBuilder) Build() []byte {
	return hclwrite.Format(b.file.Bytes())
}

// Clone returns a new cloned FileBuilder.
func (b FileBuilder) Clone() *FileBuilder {
	return New().SetContent(b.Build())
}

func (b *FileBuilder) At(addr string, f func(Builder)) *FileBuilder {
	bb, err := atAddress(b.file, addr)
	if err != nil {
		panic(err)
	}
	f(bb)
	return b
}

func (b *FileBuilder) SetAttribute(name string, src []byte) *FileBuilder {
	if diags := b.body.SetAttribute(name, src); diags.HasErrors() {
		panic(diags.Error())
	}
	return b
}

func (b *FileBuilder) RenameAttribute(fromName, toName string) *FileBuilder {
	b.body.RenameAttribute(fromName, toName)
	return b
}

func (b *FileBuilder) RemoveAttribute(name string) *FileBuilder {
	b.body.RemoveAttribute(name)
	return b
}

// AppendBlock appends a block to the end of the body in verbatim.
func (b *FileBuilder) AppendBlock(src []byte) *FileBuilder {
	if diags := b.body.AppendBlock(src); diags.HasErrors() {
		panic(diags.Error())
	}
	return b
}

// AppendNewBlock appends a new nested block to the end of the receiving body
// with the given type name and labels. Then callback the function with a newly
// constructed FileBuilder for this appended block.
func (b *FileBuilder) AppendNewBlock(typeName string, labels []string, f func(*BlockBuilder)) *FileBuilder {
	b.body.AppendNewBlock(typeName, labels, f)
	return b
}

// RemoveBlocks removes the blocks with certain type and labels.
// If indicies is not nil, only the blocks under the specified indicies are removed.
// Otherwise, all matching blocks are removed.
func (b *FileBuilder) RemoveBlocks(typeName string, labels []string, indicies []int) *FileBuilder {
	b.body.RemoveBlocks(typeName, labels, indicies)
	return b
}

type BlockBuilder struct {
	blk  *hclwrite.Block
	body bodyOperator
}

func NewBlockBuilder(blk *hclwrite.Block) *BlockBuilder {
	return &BlockBuilder{
		blk:  blk,
		body: bodyOperator{body: blk.Body()},
	}
}

var _ Builder = &BlockBuilder{}

func (b *BlockBuilder) SetType(typeName string) *BlockBuilder {
	b.blk.SetType(typeName)
	return b
}

func (b *BlockBuilder) SetLabels(labels []string) *BlockBuilder {
	b.blk.SetLabels(labels)
	return b
}

func (b *BlockBuilder) At(addr string, f func(Builder)) *BlockBuilder {
	bb, err := atAddress(b.blk, addr)
	if err != nil {
		panic(err)
	}
	f(bb)
	return b
}

func (b *BlockBuilder) SetAttribute(name string, src []byte) *BlockBuilder {
	if diags := b.body.SetAttribute(name, src); diags.HasErrors() {
		panic(diags.Error())
	}
	return b
}

func (b *BlockBuilder) RenameAttribute(fromName, toName string) *BlockBuilder {
	b.body.RenameAttribute(fromName, toName)
	return b
}

func (b *BlockBuilder) RemoveAttribute(name string) *BlockBuilder {
	b.body.RemoveAttribute(name)
	return b
}

// AppendBlock appends a block to the end of the body in verbatim.
func (b *BlockBuilder) AppendBlock(src []byte) *BlockBuilder {
	if diags := b.body.AppendBlock(src); diags.HasErrors() {
		panic(diags.Error())
	}
	return b
}

// AppendNewBlock appends a new nested block to the end of the receiving body
// with the given type name and labels. Then callback the function with a newly
// constructed BlockBuilder for this appended block.
func (b *BlockBuilder) AppendNewBlock(typeName string, labels []string, f func(*BlockBuilder)) *BlockBuilder {
	b.body.AppendNewBlock(typeName, labels, f)
	return b
}

// RemoveBlocks removes the blocks with certain type and labels.
// If indicies is not nil, only the blocks under the specified indicies are removed.
// Otherwise, all matching blocks are removed.
func (b *BlockBuilder) RemoveBlocks(typeName string, labels []string, indicies []int) *BlockBuilder {
	b.body.RemoveBlocks(typeName, labels, indicies)
	return b
}

func (b *BlockBuilder) AsBlock() *BlockBuilder {
	return b
}

func (b *BlockBuilder) AsObject() *ObjectBuilder {
	return nil
}

type ObjectBuilder struct {
	obj *hclwrite.ObjectConsExpr
}

func NewObjectBuilder(obj *hclwrite.ObjectConsExpr) *ObjectBuilder {
	return &ObjectBuilder{obj: obj}
}

var _ Builder = &ObjectBuilder{}

func (b *ObjectBuilder) At(addr string, f func(Builder)) *ObjectBuilder {
	bb, err := atAddress(b.obj, addr)
	if err != nil {
		panic(err)
	}
	f(bb)
	return b
}

func (o *ObjectBuilder) SetItem(key string, src []byte) *ObjectBuilder {
	expr, diags := hclwrite.ParseExpression(src, "", hcl.InitialPos)
	if diags.HasErrors() {
		panic(diags.Error())
	}
	o.obj.SetItem(key, expr)
	return o
}

func (o *ObjectBuilder) AsBlock() *BlockBuilder {
	return nil
}

func (o *ObjectBuilder) AsObject() *ObjectBuilder {
	return o
}

type bodyOperator struct {
	body *hclwrite.Body
}

func (b bodyOperator) SetAttribute(name string, src []byte) hcl.Diagnostics {
	expr, diags := hclwrite.ParseExpression(src, "", hcl.InitialPos)
	if diags.HasErrors() {
		return diags
	}
	b.body.SetAttribute(name, expr)
	return diags
}

func (b bodyOperator) RenameAttribute(fromName, toName string) {
	b.body.RenameAttribute(fromName, toName)
}

func (b bodyOperator) RemoveAttribute(name string) {
	b.body.RemoveAttribute(name)
}

func (b bodyOperator) AppendBlock(src []byte) hcl.Diagnostics {
	f, diags := hclwrite.ParseConfig(src, "", hcl.InitialPos)
	if diags.HasErrors() {
		return diags
	}

	// Sanity check
	if l := len(f.Body().Blocks()); l != 1 {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf("exact one block is expected, got=%d", l),
		})
	}
	if len(f.Body().Attributes()) != 0 {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "exact one block is expected, unexpected attributes got",
		})
	}
	if diags.HasErrors() {
		return diags
	}

	b.body.AppendBlock(f.Body().Blocks()[0])
	return diags
}

func (b bodyOperator) AppendNewBlock(typeName string, labels []string, f func(*BlockBuilder)) {
	innerBlk := b.body.AppendNewBlock(typeName, labels)
	f(NewBlockBuilder(innerBlk))
}

func (b bodyOperator) RemoveBlocks(typeName string, labels []string, indicies []int) {
	idx := -1
	for _, blk := range b.body.Blocks() {
		if !(blk.Type() == typeName && slices.Equal(blk.Labels(), labels)) {
			continue
		}
		idx++
		if indicies == nil || slices.Contains(indicies, idx) {
			b.body.RemoveBlock(blk)
		}
	}
}

func atAddress(start Node, address string) (Builder, error) {
	addr, err := ParseAddress(address)
	if err != nil {
		return nil, err
	}

	node := start
	for _, step := range addr {
		node, err = atStep(node, step)
		if err != nil {
			return nil, err
		}
	}

	// TODO: Support TupleBuilder
	switch node := node.(type) {
	case *hclwrite.Block:
		return NewBlockBuilder(node), nil
	case *hclwrite.ObjectConsExpr:
		return NewObjectBuilder(node), nil
	default:
		panic(fmt.Sprintf("unexpected node type %T", node))
	}
}

func atStep(node Node, step Step) (Node, error) {
	switch step := step.(type) {
	case BlockStep:
		var body *hclwrite.Body
		switch node := node.(type) {
		case *hclwrite.File:
			body = node.Body()
		case *hclwrite.Block:
			body = node.Body()
		default:
			return nil, fmt.Errorf("invalid starting node (%T) for step %q", node, step)
		}

		n := 0
		if step.Idx != nil {
			n = *step.Idx
		}
		for _, blk := range body.Blocks() {
			if !(blk.Type() == step.Type && slices.Equal(blk.Labels(), step.Labels)) {
				continue
			}
			if n != 0 {
				n--
				continue
			}
			return blk, nil
		}
		return nil, fmt.Errorf("node not found at step %q", step)
	case KeyStep:
		switch node := node.(type) {
		case *hclwrite.File:
			attr := node.Body().GetAttribute(step.Key)
			if attr == nil {
				return nil, fmt.Errorf("node not found at step %q", step)
			}
			return underlyingExpr(attr.Expr())
		case *hclwrite.Block:
			attr := node.Body().GetAttribute(step.Key)
			if attr == nil {
				return nil, fmt.Errorf("node not found at step %q", step)
			}
			return underlyingExpr(attr.Expr())
		case *hclwrite.ObjectConsExpr:
			item := node.ItemFor(step.Key)
			if item == nil {
				return nil, fmt.Errorf("node not found at step %q", step)
			}
			return underlyingExpr(item.ValueObj().Expr())
		default:
			return nil, fmt.Errorf("invalid starting node (%T) for step %q", node, step)
		}
	case IndexStep:
		// TODO: add support for TupleConsExpr once available
		switch node := node.(type) {
		default:
			return nil, fmt.Errorf("invalid starting node (%T) for step %q", node, step)
		}
	default:
		panic("unreachable")
	}
}

func underlyingExpr(expr *hclwrite.Expression) (Node, error) {
	// TODO: Add support for other expressions once available (incl. TupleConsExpr)
	switch {
	case expr.AsObjectConsExpr() != nil:
		return expr.AsObjectConsExpr(), nil
	case expr.AsQuotedLiteral() != nil:
		return expr.AsQuotedLiteral(), nil
	default:
		return nil, fmt.Errorf("unsupported expression type %s", spew.Sdump(expr))
	}
}
