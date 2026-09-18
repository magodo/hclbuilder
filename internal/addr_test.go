package internal

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseAddress(t *testing.T) {
	inputs := []struct {
		addr   string
		expect Address
		error  bool
	}{
		{
			addr:   "",
			expect: nil,
		},
		{
			addr:   "a",
			expect: Address{KeyStep{Key: "a"}},
		},
		{
			addr:   "1",
			expect: Address{IndexStep{Idx: 1}},
		},
		{
			addr: "a.1.1.a",
			expect: Address{
				KeyStep{Key: "a"},
				IndexStep{Idx: 1},
				IndexStep{Idx: 1},
				KeyStep{Key: "a"},
			},
		},
		{
			addr: "[a]",
			expect: Address{
				BlockStep{Type: "a"},
			},
		},
		{
			addr: "[a.0]",
			expect: Address{
				BlockStep{Type: "a", Idx: new(0)},
			},
		},
		{
			addr: `[a."0"]`,
			expect: Address{
				BlockStep{Type: "a", Labels: []string{"0"}},
			},
		},
		{
			addr: `[a.0.1.2]`,
			expect: Address{
				BlockStep{Type: "a", Labels: []string{"0", "1"}, Idx: new(2)},
			},
		},
		{
			addr: `[a.0.1."2"]`,
			expect: Address{
				BlockStep{Type: "a", Labels: []string{"0", "1", "2"}},
			},
		},
		{
			addr: "[a.b]",
			expect: Address{
				BlockStep{Type: "a", Labels: []string{"b"}},
			},
		},
		{
			addr: "[a.b.c.0]",
			expect: Address{
				BlockStep{Type: "a", Labels: []string{"b", "c"}, Idx: new(0)},
			},
		},
		{
			addr: "[a.b.c.0].[x.y].j.0.k.0",
			expect: Address{
				BlockStep{Type: "a", Labels: []string{"b", "c"}, Idx: new(0)},
				BlockStep{Type: "x", Labels: []string{"y"}},
				KeyStep{Key: "j"},
				IndexStep{Idx: 0},
				KeyStep{Key: "k"},
				IndexStep{Idx: 0},
			},
		},
		{
			addr:  "[]",
			error: true,
		},
		{
			addr:  "[a",
			error: true,
		},
		{
			addr:  "[.]",
			error: true,
		},
		{
			addr:  "a.",
			error: true,
		},
		{
			addr:  ".",
			error: true,
		},
	}

	for _, tt := range inputs {
		t.Run(tt.addr, func(t *testing.T) {
			actual, err := ParseAddress(tt.addr)
			if tt.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expect, actual)
				require.Equal(t, tt.addr, actual.String())
			}
		})
	}
}
