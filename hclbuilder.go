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

type NodeBuilder interface {
	AsBlock() *BlockBuilder
	AsObject() *ObjectBuilder
	// TODO: Add AsTuple when TupleConsExpr is available
}

type Builder struct {
	file *hclwrite.File
	body bodyOperator

	ef ErrorFunc
}

// New creates a Builder with the input content.
// Any build operation that failed will cause a panic by default, which can be
// changed by setting a custom error function via WithErrorFunc option.
func New(src []byte, opts ...Option) *Builder {
	b := &Builder{
		ef: func(err error) { panic(err.Error()) },
	}
	for _, opt := range opts {
		opt(b)
	}

	file := hclwrite.NewEmptyFile()
	if len(src) != 0 {
		var diags hcl.Diagnostics
		file, diags = hclwrite.ParseConfig(src, "", hcl.InitialPos)
		if onDiags(b.ef, diags) {
			return nil
		}
	}

	b.file = file
	b.body = bodyOperator{
		body: file.Body(),
		ef:   b.ef,
	}
	return b
}

// Build turns the HCL built so far into formatted bytes.
func (b Builder) Build() []byte {
	return hclwrite.Format(b.file.Bytes())
}

// BuildString is similar to Build, but returns string.
func (b Builder) BuildString() string {
	return string(b.Build())
}

// Clone returns a new cloned FileBuilder.
func (b Builder) Clone() *Builder {
	bb := New(b.Build())
	bb.ef = b.ef
	return bb
}

// At goes down to the addr and apply the build function with the builder at that level.
// The build function shall convert the Builder to a concrete builder via the AsXXX method.
//
// The format of the addr is dot separated steps, where each step can be one of the below:
// - block step: "[" blk_type(.blk_label1.blk_label2,...)(.index)? "]" (index defaults to 0)
// - key step: An identifier represents the attribute name or object's key.
// - index step: A number represents the index of a tuple.
//
// Example: With addr [resource.azurerm_resource_group.test].tags, a ObjectBuilder is called
// with the build function.
func (b *Builder) At(addr string, f func(NodeBuilder)) *Builder {
	bb, err := atAddress(b.file, addr, b.ef)
	if onErr(b.ef, err) {
		return b
	}
	f(bb)
	return b
}

func (b *Builder) SetAttribute(name string, content string) *Builder {
	onDiags(b.ef, b.body.SetAttribute(name, []byte(content)))
	return b
}

func (b *Builder) RenameAttribute(fromName, toName string) *Builder {
	onBool(b.ef, b.body.RenameAttribute(fromName, toName), fmt.Sprintf("RenameAttribute for %s failed", fromName))
	return b
}

func (b *Builder) RemoveAttribute(name string) *Builder {
	onBool(b.ef, b.body.RemoveAttribute(name), fmt.Sprintf("RemoveAttribute for %s failed", name))
	return b
}

// AppendBlock appends a block to the end of the body in verbatim.
func (b *Builder) AppendBlock(content string) *Builder {
	onDiags(b.ef, b.body.AppendBlock([]byte(content)))
	return b
}

// AppendNewBlock appends a new nested block to the end of the receiving body
// with the given type name and labels. Then callback the function with a newly
// constructed FileBuilder for this appended block.
func (b *Builder) AppendNewBlock(typeName string, labels []string, f func(*BlockBuilder)) *Builder {
	b.body.AppendNewBlock(typeName, labels, f)
	return b
}

// RemoveBlocks removes the blocks with certain type and labels.
// If indicies is not nil, only the blocks under the specified indicies are removed.
// Otherwise, all matching blocks are removed.
func (b *Builder) RemoveBlocks(typeName string, labels []string, indicies []int) *Builder {
	onErr(b.ef, b.body.RemoveBlocks(typeName, labels, indicies))
	return b
}

type BlockBuilder struct {
	blk  *hclwrite.Block
	body bodyOperator

	ef ErrorFunc
}

func NewBlockBuilder(blk *hclwrite.Block, ef ErrorFunc) *BlockBuilder {
	return &BlockBuilder{
		blk:  blk,
		body: bodyOperator{body: blk.Body(), ef: ef},

		ef: ef,
	}
}

var _ NodeBuilder = &BlockBuilder{}

func (b *BlockBuilder) SetType(typeName string) *BlockBuilder {
	b.blk.SetType(typeName)
	return b
}

func (b *BlockBuilder) SetLabels(labels []string) *BlockBuilder {
	b.blk.SetLabels(labels)
	return b
}

func (b *BlockBuilder) At(addr string, f func(NodeBuilder)) *BlockBuilder {
	bb, err := atAddress(b.blk, addr, b.ef)
	if onErr(b.ef, err) {
		return b
	}
	f(bb)
	return b
}

func (b *BlockBuilder) SetAttribute(name string, content string) *BlockBuilder {
	onDiags(b.ef, b.body.SetAttribute(name, []byte(content)))
	return b
}

func (b *BlockBuilder) RenameAttribute(fromName, toName string) *BlockBuilder {
	onBool(b.ef, b.body.RenameAttribute(fromName, toName), fmt.Sprintf("RenameAttribute for %s failed", fromName))
	return b
}

func (b *BlockBuilder) RemoveAttribute(name string) *BlockBuilder {
	onBool(b.ef, b.body.RemoveAttribute(name), fmt.Sprintf("RemoveAttribute for %s failed", name))
	return b
}

// AppendBlock appends a block to the end of the body in verbatim.
func (b *BlockBuilder) AppendBlock(content string) *BlockBuilder {
	onDiags(b.ef, b.body.AppendBlock([]byte(content)))
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
	onErr(b.ef, b.body.RemoveBlocks(typeName, labels, indicies))
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

	ef ErrorFunc
}

func NewObjectBuilder(obj *hclwrite.ObjectConsExpr, ef ErrorFunc) *ObjectBuilder {
	return &ObjectBuilder{obj: obj, ef: ef}
}

var _ NodeBuilder = &ObjectBuilder{}

func (b *ObjectBuilder) At(addr string, f func(NodeBuilder)) *ObjectBuilder {
	bb, err := atAddress(b.obj, addr, b.ef)
	if onErr(b.ef, err) {
		return b
	}
	f(bb)
	return b
}

func (b *ObjectBuilder) SetItem(key string, content string) *ObjectBuilder {
	expr, diags := hclwrite.ParseExpression([]byte(content), "", hcl.InitialPos)
	if onDiags(b.ef, diags) {
		return b
	}
	b.obj.SetItem(key, expr)
	return b
}

func (b *ObjectBuilder) RemoveItem(key string) *ObjectBuilder {
	onBool(b.ef, b.obj.RemoveItem(key), fmt.Sprintf("RemoveItem for %s failed", key))
	return b
}

func (b *ObjectBuilder) AsBlock() *BlockBuilder {
	return nil
}

func (b *ObjectBuilder) AsObject() *ObjectBuilder {
	return b
}

type bodyOperator struct {
	body *hclwrite.Body

	ef ErrorFunc
}

func (b bodyOperator) SetAttribute(name string, src []byte) hcl.Diagnostics {
	expr, diags := hclwrite.ParseExpression(src, "", hcl.InitialPos)
	if diags.HasErrors() {
		return diags
	}
	b.body.SetAttribute(name, expr)
	return diags
}

func (b bodyOperator) RenameAttribute(fromName, toName string) bool {
	return b.body.RenameAttribute(fromName, toName)
}

func (b bodyOperator) RemoveAttribute(name string) bool {
	return b.body.RemoveAttribute(name) != nil
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
	f(NewBlockBuilder(innerBlk, b.ef))
}

func (b bodyOperator) RemoveBlocks(typeName string, labels []string, indicies []int) error {
	idx := -1
	for _, blk := range b.body.Blocks() {
		if !(blk.Type() == typeName && slices.Equal(blk.Labels(), labels)) {
			continue
		}
		idx++
		if indicies == nil || slices.Contains(indicies, idx) {
			if !b.body.RemoveBlock(blk) {
				return fmt.Errorf("RemoveBlock for %s failed", BlockStep{Type: typeName, Labels: labels})
			}
		}
	}
	return nil
}

func atAddress(start Node, address string, ef ErrorFunc) (NodeBuilder, error) {
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
		return NewBlockBuilder(node, ef), nil
	case *hclwrite.ObjectConsExpr:
		return NewObjectBuilder(node, ef), nil
	default:
		return nil, fmt.Errorf("unexpected node type %T", node)
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
