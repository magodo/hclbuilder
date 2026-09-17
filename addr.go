package hclbuilder

import (
	"bufio"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
)

const (
	BlockOpen  = '['
	BlockClose = ']'
	StepSep    = '.'
)

type Address []Step

func (addr Address) String() string {
	var segs []string
	for _, step := range addr {
		segs = append(segs, step.String())
	}
	return strings.Join(segs, string(StepSep))
}

type Step interface {
	isStep()
	String() string
}

type BlockStep struct {
	Type   string
	Labels []string
	Idx    *int
}

func (BlockStep) isStep() {}

// String returns the string representation of the BlockStep.
// It will not quote any label unless it is the last segment and is a number.
func (step BlockStep) String() string {
	segs := append([]string{step.Type}, step.Labels...)
	if step.Idx != nil {
		segs = append(segs, strconv.Itoa(*step.Idx))
	} else {
		if len(step.Labels) > 0 {
			last := step.Labels[len(step.Labels)-1]
			if _, err := strconv.Atoi(last); err == nil {
				segs[len(segs)-1] = strconv.Quote(last)
			}
		}
	}
	return fmt.Sprintf("%s%s%s", string(BlockOpen), strings.Join(segs, string(StepSep)), string(BlockClose))
}

type KeyStep struct {
	Key string
}

func (KeyStep) isStep() {}

func (step KeyStep) String() string {
	return step.Key
}

type IndexStep struct {
	Idx int
}

func (IndexStep) isStep() {}

func (step IndexStep) String() string {
	return strconv.Itoa(step.Idx)
}

// ParseAddress parses a string into an Address.
// The grammer of the address is:
//
//		Address -> Step(.Step)*
//
//	 Step ->
//		BlockStep |
//		Identifier |
//		Number
//
//	  BlockStep -> "[" BlockType ("." BlockLabel)* ("." BlockIndex)?  "]"
//
//	  BlockLabel can be quoted by `"` (e.g. to represent a number-like string).
//	  The BlockStep must go before any other step.
//
// For now, we use a lex-less implementation.
func ParseAddress(input string) (Address, error) {
	buf := bufio.NewReader(strings.NewReader(input))

	// HCL can only have blocks prior to other constructions, not the other way around.
	// Hence we read all blocks first.
	var addr Address
	for {
		bytes, err := buf.Peek(1)
		if err != nil {
			if err == io.EOF {
				break
			}
			return addr, err
		}

		next := bytes[0]
		if next != BlockOpen {
			break
		}

		blkInput, err := buf.ReadString(BlockClose)
		if err != nil {
			return addr, fmt.Errorf("failed to read until the %q: %v", string(BlockClose), err)
		}
		step, err := parseStepBlock(blkInput[1 : len(blkInput)-1])
		if err != nil {
			return addr, err
		}
		addr = append(addr, step)

		next, err = buf.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			}
		}
		if next != StepSep {
			return addr, fmt.Errorf("invalid step separator: %v", string(next))
		}
	}

	remaining, err := io.ReadAll(buf)
	if err != nil {
		if err == io.EOF {
			return addr, nil
		}
		return addr, err
	}

	if len(remaining) == 0 {
		return addr, nil
	}

	for seg := range strings.SplitSeq(string(remaining), string(StepSep)) {
		if seg == "" {
			return addr, fmt.Errorf("empty step segment is not allowed")
		}
		if n, err := strconv.Atoi(seg); err == nil {
			addr = append(addr, IndexStep{Idx: n})
		} else {
			addr = append(addr, KeyStep{Key: seg})
		}
	}

	return addr, nil
}

func parseStepBlock(input string) (BlockStep, error) {
	if input == "" {
		return BlockStep{}, fmt.Errorf("empty block step input")
	}

	segs := strings.Split(input, string(StepSep))
	if slices.Contains(segs, "") {
		return BlockStep{}, fmt.Errorf("empty step segment is not allowed")
	}

	step := BlockStep{
		Type: segs[0],
	}
	if len(segs) > 1 {
		for _, seg := range segs[1 : len(segs)-1] {
			if v, err := strconv.Unquote(seg); err == nil {
				seg = v
			}
			step.Labels = append(step.Labels, seg)
		}

		last := segs[len(segs)-1]
		if v, err := strconv.Unquote(last); err == nil {
			step.Labels = append(step.Labels, v)
		} else if n, err := strconv.Atoi(last); err == nil {
			step.Idx = &n
		} else {
			step.Labels = append(step.Labels, last)
		}
	}

	return step, nil
}
