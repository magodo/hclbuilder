package hclbuilder

import (
	"fmt"
	"slices"

	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/magodo/hclbuilder/internal"
)

// Node is an interface implemented by all the hclwrite nodes.
type Node interface {
	// token sequence.
	BuildTokens(dst hclwrite.Tokens) hclwrite.Tokens
}

// Builder provides access to the builder for an HCL node.
// Exactly one of AsFile, AsBlock, AsObject, and AsTuple returns non-nil,
// corresponding to the node's kind.
type Builder interface {
	// AsFile returns a FileBuilder when the builder represents a file, or nil
	// otherwise.
	AsFile() *FileBuilder
	// AsBlock returns a BlockBuilder when the builder represents a block, or
	// nil otherwise.
	AsBlock() *BlockBuilder
	// AsObject returns an ObjectBuilder when the builder represents an object,
	// or nil otherwise.
	AsObject() *ObjectBuilder
	// AsTuple returns a TupleBuilder when the builder represents a tuple, or
	// nil otherwise.
	AsTuple() *TupleBuilder
}

// FileBuilder builds and modifies an HCL file.
type FileBuilder struct {
	file *hclwrite.File
	body bodyOperator

	ef ErrorFunc
}

func newFileBuilder(file *hclwrite.File, ef ErrorFunc) *FileBuilder {
	return &FileBuilder{
		file: file,
		body: bodyOperator{
			body: file.Body(),
			ef:   ef,
		},
		ef: ef,
	}
}

var _ Builder = &FileBuilder{}

// New creates a FileBuilder with the input content.
// Any build operation that failed will cause a panic by default, which can be
// changed by setting a custom error function via WithErrorFunc option.
func New(src []byte, opts ...Option) *FileBuilder {
	// Construct a tmp FileBuilder mainly to apply the options.
	// It will be replaced by another new FileBuilder constructed below.
	tmp := &FileBuilder{
		ef: func(err error) { panic(err.Error()) },
	}
	for _, opt := range opts {
		opt(tmp)
	}

	file := hclwrite.NewEmptyFile()
	if len(src) != 0 {
		var diags hcl.Diagnostics
		file, diags = hclwrite.ParseConfig(src, "", hcl.InitialPos)
		if onDiags(tmp.ef, diags) {
			return nil
		}
	}

	return newFileBuilder(file, tmp.ef)
}

// Build turns the HCL built so far into formatted bytes.
func (b FileBuilder) Build() []byte {
	return hclwrite.Format(b.file.Bytes())
}

// BuildString is similar to Build, but returns string.
func (b FileBuilder) BuildString() string {
	return string(b.Build())
}

// Clone returns a new cloned FileBuilder.
func (b FileBuilder) Clone() *FileBuilder {
	bb := New(b.Build())
	bb.ef = b.ef
	return bb
}

// At calls f with the builder for the node at addr.
func (b *FileBuilder) At(addr string, f func(Builder)) *FileBuilder {
	bb, err := atAddress(b.file, addr, b.ef)
	if onErr(b.ef, err) {
		return b
	}
	f(bb)
	return b
}

// SetAttribute sets name to the HCL expression in content.
func (b *FileBuilder) SetAttribute(name string, content string) *FileBuilder {
	onDiags(b.ef, b.body.SetAttribute(name, []byte(content)))
	return b
}

// RenameAttribute renames an attribute from fromName to toName.
func (b *FileBuilder) RenameAttribute(fromName, toName string) *FileBuilder {
	onBool(b.ef, b.body.RenameAttribute(fromName, toName), fmt.Sprintf("RenameAttribute for %s failed", fromName))
	return b
}

// RemoveAttribute removes the attribute named name.
func (b *FileBuilder) RemoveAttribute(name string) *FileBuilder {
	onBool(b.ef, b.body.RemoveAttribute(name), fmt.Sprintf("RemoveAttribute for %s failed", name))
	return b
}

// AppendBlock appends a block to the end of the body.
func (b *FileBuilder) AppendBlock(content string) *FileBuilder {
	onDiags(b.ef, b.body.AppendBlock([]byte(content)))
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
	onErr(b.ef, b.body.RemoveBlocks(typeName, labels, indicies))
	return b
}

// SetExpressionAt sets the raw content of an expression to where the address points to.
func (b *FileBuilder) SetExpressionAt(addr, content string) *FileBuilder {
	parentNode, last, err := b.resolveParent(addr)
	if onErr(b.ef, err) {
		return b
	}

	step, ok := last.(internal.KeyStep)
	if !ok {
		onErr(b.ef, fmt.Errorf("cannot set %q: address is not pointing to an attribute or object", addr))
		return b
	}

	switch {
	case parentNode.AsFile() != nil:
		parentNode.AsFile().SetAttribute(step.Key, content)
	case parentNode.AsBlock() != nil:
		parentNode.AsBlock().SetAttribute(step.Key, content)
	case parentNode.AsObject() != nil:
		parentNode.AsObject().SetItem(step.Key, content)
	default:
		onErr(b.ef, fmt.Errorf("cannot set %q: parent is not a block or object body", addr))
	}

	return b
}

// AppendBlockAt appends the raw content of a block to the body at addr.
// An empty address appends the block to the root file body.
func (b *FileBuilder) AppendBlockAt(addr, content string) *FileBuilder {
	parentNode, err := atAddress(b.file, addr, b.ef)
	if onErr(b.ef, err) {
		return b
	}

	switch {
	case parentNode.AsFile() != nil:
		parentNode.AsFile().AppendBlock(content)
	case parentNode.AsBlock() != nil:
		parentNode.AsBlock().AppendBlock(content)
	default:
		onErr(b.ef, fmt.Errorf("cannot append a block at %q: not a file or block body", addr))
	}
	return b
}

// RemoveAt removes the node where the address points to.
func (b *FileBuilder) RemoveAt(addr string) *FileBuilder {
	parentNode, last, err := b.resolveParent(addr)
	if onErr(b.ef, err) {
		return b
	}

	// TODO: Support IndexStep once TupleConsExpr can remove item.
	switch step := last.(type) {
	case internal.KeyStep:
		switch {
		case parentNode.AsFile() != nil:
			parentNode.AsFile().RemoveAttribute(step.Key)
		case parentNode.AsBlock() != nil:
			parentNode.AsBlock().RemoveAttribute(step.Key)
		case parentNode.AsObject() != nil:
			parentNode.AsObject().RemoveItem(step.Key)
		default:
			onErr(b.ef, fmt.Errorf("cannot remove %q: parent is not a file, block or object", addr))
		}
	case internal.BlockStep:
		idx := []int{0}
		if step.Idx != nil {
			idx = []int{*step.Idx}
		}
		switch {
		case parentNode.AsFile() != nil:
			parentNode.AsFile().RemoveBlocks(step.Type, step.Labels, idx)
		case parentNode.AsBlock() != nil:
			parentNode.AsBlock().RemoveBlocks(step.Type, step.Labels, idx)
		default:
			onErr(b.ef, fmt.Errorf("cannot remove a block at %q: parent is not a file or block", addr))
		}
	default:
		onErr(b.ef, fmt.Errorf("unsupported final step %q", last))
	}
	return b
}

// AsBlock returns nil.
func (b *FileBuilder) AsBlock() *BlockBuilder {
	return nil
}

// AsFile returns the receiver.
func (b *FileBuilder) AsFile() *FileBuilder {
	return b
}

// AsObject returns nil.
func (b *FileBuilder) AsObject() *ObjectBuilder {
	return nil
}

// AsTuple returns nil.
func (b *FileBuilder) AsTuple() *TupleBuilder {
	return nil
}

// resolveParent resolves the parent node, with the last address step.
func (b *FileBuilder) resolveParent(addr string) (parentNode Builder, last internal.Step, err error) {
	address, err := internal.ParseAddress(addr)
	if err != nil {
		return nil, nil, err
	}
	if len(address) == 0 {
		return nil, nil, fmt.Errorf("empty address is not allowed")
	}
	parent, last := address[:len(address)-1], address[len(address)-1]
	parentNode, err = atAddress(b.file, parent.String(), b.ef)
	return parentNode, last, err
}

// BlockBuilder builds and modifies an HCL block.
type BlockBuilder struct {
	blk  *hclwrite.Block
	body bodyOperator

	ef ErrorFunc
}

func newBlockBuilder(blk *hclwrite.Block, ef ErrorFunc) *BlockBuilder {
	return &BlockBuilder{
		blk:  blk,
		body: bodyOperator{body: blk.Body(), ef: ef},

		ef: ef,
	}
}

var _ Builder = &BlockBuilder{}

// SetType sets the block type to typeName.
func (b *BlockBuilder) SetType(typeName string) *BlockBuilder {
	b.blk.SetType(typeName)
	return b
}

// SetLabels sets the block labels to labels.
func (b *BlockBuilder) SetLabels(labels []string) *BlockBuilder {
	b.blk.SetLabels(labels)
	return b
}

// At calls f with the builder for the node at addr.
func (b *BlockBuilder) At(addr string, f func(Builder)) *BlockBuilder {
	bb, err := atAddress(b.blk, addr, b.ef)
	if onErr(b.ef, err) {
		return b
	}
	f(bb)
	return b
}

// SetAttribute sets name to the HCL expression in content.
func (b *BlockBuilder) SetAttribute(name string, content string) *BlockBuilder {
	onDiags(b.ef, b.body.SetAttribute(name, []byte(content)))
	return b
}

// RenameAttribute renames an attribute from fromName to toName.
func (b *BlockBuilder) RenameAttribute(fromName, toName string) *BlockBuilder {
	onBool(b.ef, b.body.RenameAttribute(fromName, toName), fmt.Sprintf("RenameAttribute for %s failed", fromName))
	return b
}

// RemoveAttribute removes the attribute named name.
func (b *BlockBuilder) RemoveAttribute(name string) *BlockBuilder {
	onBool(b.ef, b.body.RemoveAttribute(name), fmt.Sprintf("RemoveAttribute for %s failed", name))
	return b
}

// AppendBlock appends a block to the end of the body.
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

// AsBlock returns the receiver.
func (b *BlockBuilder) AsBlock() *BlockBuilder {
	return b
}

// AsFile returns nil.
func (b *BlockBuilder) AsFile() *FileBuilder {
	return nil
}

// AsObject returns nil.
func (b *BlockBuilder) AsObject() *ObjectBuilder {
	return nil
}

// AsTuple returns nil.
func (b *BlockBuilder) AsTuple() *TupleBuilder {
	return nil
}

// ObjectBuilder builds and modifies an HCL object constructor expression.
type ObjectBuilder struct {
	obj *hclwrite.ObjectConsExpr

	ef ErrorFunc
}

func newObjectBuilder(obj *hclwrite.ObjectConsExpr, ef ErrorFunc) *ObjectBuilder {
	return &ObjectBuilder{obj: obj, ef: ef}
}

var _ Builder = &ObjectBuilder{}

// At calls f with the builder for the node at addr.
func (b *ObjectBuilder) At(addr string, f func(Builder)) *ObjectBuilder {
	bb, err := atAddress(b.obj, addr, b.ef)
	if onErr(b.ef, err) {
		return b
	}
	f(bb)
	return b
}

// SetItem sets key to the HCL expression in content.
func (b *ObjectBuilder) SetItem(key string, content string) *ObjectBuilder {
	expr, diags := hclwrite.ParseExpression([]byte(content), "", hcl.InitialPos)
	if onDiags(b.ef, diags) {
		return b
	}
	b.obj.SetItem(key, expr)
	return b
}

// RemoveItem removes the item identified by key.
func (b *ObjectBuilder) RemoveItem(key string) *ObjectBuilder {
	onBool(b.ef, b.obj.RemoveItem(key), fmt.Sprintf("RemoveItem for %s failed", key))
	return b
}

// AsBlock returns nil.
func (b *ObjectBuilder) AsBlock() *BlockBuilder {
	return nil
}

// AsFile returns nil.
func (b *ObjectBuilder) AsFile() *FileBuilder {
	return nil
}

// AsObject returns the receiver.
func (b *ObjectBuilder) AsObject() *ObjectBuilder {
	return b
}

// AsTuple returns nil.
func (b *ObjectBuilder) AsTuple() *TupleBuilder {
	return nil
}

// TupleBuilder represents an HCL tuple constructor expression.
type TupleBuilder struct {
	tuple *hclwrite.TupleConsExpr

	ef ErrorFunc
}

func newTupleBuilder(tuple *hclwrite.TupleConsExpr, ef ErrorFunc) *TupleBuilder {
	return &TupleBuilder{tuple: tuple, ef: ef}
}

var _ Builder = &TupleBuilder{}

// AsBlock returns nil.
func (t *TupleBuilder) AsBlock() *BlockBuilder {
	return nil
}

// AsFile returns nil.
func (t *TupleBuilder) AsFile() *FileBuilder {
	return nil
}

// AsObject returns nil.
func (t *TupleBuilder) AsObject() *ObjectBuilder {
	return nil
}

// AsTuple returns the receiver.
func (t *TupleBuilder) AsTuple() *TupleBuilder {
	return t
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

// AppendBlock build construct a block from the src and append it to the body.
func (b bodyOperator) AppendBlock(src []byte) hcl.Diagnostics {
	// Ensure the src is always ended with a newline, which avoids
	// the constructed block is followed by the next node at the same line.
	// This aligns with how AppendNewBlock does in its inner `.init()` process.
	// Multiple newlines seem to be normalized to one by hclwrite write/fmt process.
	src = append(src, '\n')

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
	f(newBlockBuilder(innerBlk, b.ef))
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
				return fmt.Errorf("RemoveBlock for %s failed", internal.BlockStep{Type: typeName, Labels: labels})
			}
		}
	}
	return nil
}

func atAddress(start Node, address string, ef ErrorFunc) (Builder, error) {
	addr, err := internal.ParseAddress(address)
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

	switch node := node.(type) {
	case *hclwrite.File:
		return newFileBuilder(node, ef), nil
	case *hclwrite.Block:
		return newBlockBuilder(node, ef), nil
	case *hclwrite.ObjectConsExpr:
		return newObjectBuilder(node, ef), nil
	case *hclwrite.TupleConsExpr:
		return newTupleBuilder(node, ef), nil
	default:
		return nil, fmt.Errorf("unexpected node type %T", node)
	}
}

func atStep(node Node, step internal.Step) (Node, error) {
	switch step := step.(type) {
	case internal.BlockStep:
		var body *hclwrite.Body
		switch node := node.(type) {
		case *hclwrite.File:
			body = node.Body()
		case *hclwrite.Block:
			body = node.Body()
		default:
			return nil, fmt.Errorf("invalid starting node (%T) for block step %q", node, step)
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
	case internal.KeyStep:
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
			return nil, fmt.Errorf("invalid starting node (%T) for key step %q", node, step)
		}
	case internal.IndexStep:
		switch node := node.(type) {
		case *hclwrite.TupleConsExpr:
			items := node.Items()
			if step.Idx >= len(items) {
				return nil, fmt.Errorf("invalid index (%d) for step %q (len=%d)", step.Idx, step, len(items))
			}
			return underlyingExpr(items[step.Idx])
		default:
			return nil, fmt.Errorf("invalid starting node (%T) for index step %q", node, step)
		}
	default:
		panic("unreachable")
	}
}

func underlyingExpr(expr *hclwrite.Expression) (Node, error) {
	switch {
	case expr.AsObjectConsExpr() != nil:
		return expr.AsObjectConsExpr(), nil
	case expr.AsTupleConsExpr() != nil:
		return expr.AsTupleConsExpr(), nil
	case len(expr.AsQuotedLiteral()) != 0:
		return expr.AsQuotedLiteral(), nil
	default:
		return nil, fmt.Errorf("unsupported expression type %s", spew.Sdump(expr))
	}
}
