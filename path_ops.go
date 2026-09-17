package hclbuilder

import "fmt"

// SetAt sets whatever the path points to regardless of type
func (b *Builder) SetAt(path, content string) {
	last, nb, ok := b.resolveParent(path)
	if !ok {
		return
	}

	switch step := last.(type) {
	case KeyStep:
		switch {
		case nb == nil:
			onDiags(b.ef, b.body.SetAttribute(step.Key, []byte(content)))
		case nb.AsBlock() != nil:
			nb.AsBlock().SetAttribute(step.Key, content)
		case nb.AsObject() != nil:
			nb.AsObject().SetItem(step.Key, content)
		default:
			onErr(b.ef, fmt.Errorf("cannot set %q: parent is not a block or object body", path))
		}
	case BlockStep:
		switch {
		case nb == nil:
			onDiags(b.ef, b.body.AppendBlock([]byte(content)))
		case nb.AsBlock() != nil:
			nb.AsBlock().AppendBlock(content)
		default:
			onErr(b.ef, fmt.Errorf("cannot set a block at %q: not a block body", path))
		}
	default:
		onErr(b.ef, fmt.Errorf("unsupported final step %q", last))
	}
}

// RemoveAt is kill switch that destroy whatever the path points to :)
func (b *Builder) RemoveAt(path string) {
	last, nb, ok := b.resolveParent(path)
	if !ok {
		return
	}

	switch step := last.(type) {
	case KeyStep:
		switch {
		case nb == nil:
			onBool(b.ef, b.body.RemoveAttribute(step.Key), fmt.Sprintf("RemoveAttribute for %s failed", step.Key))
		case nb.AsBlock() != nil:
			nb.AsBlock().RemoveAttribute(step.Key)
		case nb.AsObject() != nil:
			nb.AsObject().RemoveItem(step.Key)
		default:
			onErr(b.ef, fmt.Errorf("cannot remove %q: parent is not a block or object body", path))
		}
	case BlockStep:
		idx := []int{0}
		if step.Idx != nil {
			idx = []int{*step.Idx}
		}
		switch {
		case nb == nil:
			onErr(b.ef, b.body.RemoveBlocks(step.Type, step.Labels, idx))
		case nb.AsBlock() != nil:
			nb.AsBlock().RemoveBlocks(step.Type, step.Labels, idx)
		default:
			onErr(b.ef, fmt.Errorf("cannot remove a block at %q: not a block body", path))
		}
	default:
		onErr(b.ef, fmt.Errorf("unsupported final step %q", last))
	}
}

// resolveParent locates the parent node with path's final step. The returned nb is
// nil when the parent node is the file
func (b *Builder) resolveParent(path string) (last Step, nb NodeBuilder, ok bool) {
	addr, err := ParseAddress(path)
	if onErr(b.ef, err) {
		return nil, nil, false
	}
	if len(addr) == 0 {
		onErr(b.ef, fmt.Errorf("empty path"))
		return nil, nil, false
	}

	last = addr[len(addr)-1]
	parent := addr[:len(addr)-1]
	if len(parent) != 0 {
		nb, err = atAddress(b.file, parent.String(), b.ef)
		if onErr(b.ef, err) {
			return nil, nil, false
		}
	}
	return last, nb, true
}
