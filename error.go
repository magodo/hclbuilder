package hclbuilder

import (
	"errors"

	"github.com/hashicorp/hcl/v2"
)

// ErrorFunc is called with the error received during the build.
type ErrorFunc func(error)

func onDiags(f ErrorFunc, diags hcl.Diagnostics) bool {
	if diags.HasErrors() && f != nil {
		f(errors.New(diags.Error()))
	}
	return diags.HasErrors()
}

func onErr(f ErrorFunc, err error) bool {
	if err != nil && f != nil {
		f(err)
	}
	return err != nil
}
