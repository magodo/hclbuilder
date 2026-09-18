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
	AsFile() *FileBuilder
	AsBlock() *BlockBuilder
	AsObject() *ObjectBuilder
	AsTuple() *TupleBuilder
}

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
func (b *FileBuilder) At(addr string, f func(Builder)) *FileBuilder {
	bb, err := atAddress(b.file, addr, b.ef)
	if onErr(b.ef, err) {
		return b
	}
	f(bb)
	return b
}

func (b *FileBuilder) SetAttribute(name string, content string) *FileBuilder {
	onDiags(b.ef, b.body.SetAttribute(name, []byte(content)))
	return b
}

func (b *FileBuilder) RenameAttribute(fromName, toName string) *FileBuilder {
	onBool(b.ef, b.body.RenameAttribute(fromName, toName), fmt.Sprintf("RenameAttribute for %s failed", fromName))
	return b
}

func (b *FileBuilder) RemoveAttribute(name string) *FileBuilder {
	onBool(b.ef, b.body.RemoveAttribute(name), fmt.Sprintf("RemoveAttribute for %s failed", name))
	return b
}

// AppendBlock appends a block to the end of the body in verbatim.
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

// SetAt sets the raw content to where the address points to.
// The caller is responsible to ensure the content being set is valid at the address.
// E.g. Setting a block to an attribute path will cause an error.
func (b *FileBuilder) SetAt(addr, content string) *FileBuilder {
	parentNode, last, err := b.resolveParent(addr)
	if onErr(b.ef, err) {
		return b
	}

	// TODO: Support IndexStep once TupleConsExpr can set/insert item.
	switch step := last.(type) {
	case KeyStep:
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
	case BlockStep:
		switch {
		case parentNode.AsFile() != nil:
			parentNode.AsFile().AppendBlock(content)
		case parentNode.AsBlock() != nil:
			parentNode.AsBlock().AppendBlock(content)
		default:
			onErr(b.ef, fmt.Errorf("cannot set a block at %q: not a block body", addr))
		}
	default:
		onErr(b.ef, fmt.Errorf("unsupported final step %q", last))
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
	case KeyStep:
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
	case BlockStep:
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

func (b *FileBuilder) AsBlock() *BlockBuilder {
	return nil
}

func (b *FileBuilder) AsFile() *FileBuilder {
	return b
}

func (b *FileBuilder) AsObject() *ObjectBuilder {
	return nil
}

func (b *FileBuilder) AsTuple() *TupleBuilder {
	return nil
}

// resolveParent resolves the parent node, with the last address step.
func (b *FileBuilder) resolveParent(addr string) (parentNode Builder, last Step, err error) {
	address, err := ParseAddress(addr)
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

func (b *BlockBuilder) SetType(typeName string) *BlockBuilder {
	b.blk.SetType(typeName)
	return b
}

func (b *BlockBuilder) SetLabels(labels []string) *BlockBuilder {
	b.blk.SetLabels(labels)
	return b
}

func (b *BlockBuilder) At(addr string, f func(Builder)) *BlockBuilder {
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

func (b *BlockBuilder) AsFile() *FileBuilder {
	return nil
}

func (b *BlockBuilder) AsObject() *ObjectBuilder {
	return nil
}

func (b *BlockBuilder) AsTuple() *TupleBuilder {
	return nil
}

type ObjectBuilder struct {
	obj *hclwrite.ObjectConsExpr

	ef ErrorFunc
}

func newObjectBuilder(obj *hclwrite.ObjectConsExpr, ef ErrorFunc) *ObjectBuilder {
	return &ObjectBuilder{obj: obj, ef: ef}
}

var _ Builder = &ObjectBuilder{}

func (b *ObjectBuilder) At(addr string, f func(Builder)) *ObjectBuilder {
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

func (b *ObjectBuilder) AsFile() *FileBuilder {
	return nil
}

func (b *ObjectBuilder) AsObject() *ObjectBuilder {
	return b
}

func (b *ObjectBuilder) AsTuple() *TupleBuilder {
	return nil
}

type TupleBuilder struct {
	tuple *hclwrite.TupleConsExpr

	ef ErrorFunc
}

func newTupleBuilder(tuple *hclwrite.TupleConsExpr, ef ErrorFunc) *TupleBuilder {
	return &TupleBuilder{tuple: tuple, ef: ef}
}

var _ Builder = &TupleBuilder{}

func (t *TupleBuilder) AsBlock() *BlockBuilder {
	return nil
}

func (b *TupleBuilder) AsFile() *FileBuilder {
	return nil
}

func (t *TupleBuilder) AsObject() *ObjectBuilder {
	return nil
}

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
				return fmt.Errorf("RemoveBlock for %s failed", BlockStep{Type: typeName, Labels: labels})
			}
		}
	}
	return nil
}

func atAddress(start Node, address string, ef ErrorFunc) (Builder, error) {
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
			return nil, fmt.Errorf("invalid starting node (%T) for key step %q", node, step)
		}
	case IndexStep:
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
