// Copyright 2020 Rancher Labs, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cli

import (
	"flag"
	"time"

	"github.com/rancher/spur/generic"
)

var _ = time.Time{}

// GenericValue takes a pointer to a generic type
type GenericValue struct {
	ptr interface{}
	set bool
}

// NewGenericValue returns a flag.Value given a pointer
func NewGenericValue(ptr interface{}) flag.Value {
	generic.PtrPanic(ptr)
	return &GenericValue{ptr: ptr}
}

// Get returns the contents of the stored pointer
func (v *GenericValue) Get() interface{} {
	return generic.ValueOfPtr(v.ptr)
}

// Set will convert a given value to the type of our pointer
// and store the new value
func (v *GenericValue) Set(value string) error {
	return v.Apply(value)
}

// Apply will convert a given value to the type of our pointer
// and store the new value
func (v *GenericValue) Apply(value interface{}) error {
	if generic.IsSlice(v.Get()) && !v.set {
		// If this is a slice and has not already been set then
		// clear any existing value
		generic.Set(v.ptr, generic.Zero(v.Get()))
		v.set = true
	}
	val, err := generic.Convert(v.Get(), value)
	if err != nil {
		return err
	}
	generic.Set(v.ptr, val)
	return nil
}

// String returns a string representation of our generic value
func (v *GenericValue) String() string {
	return generic.Stringify(v.Get())
}

// IsBoolFlag returns true if the pointer type is bool
func (v *GenericValue) IsBoolFlag() bool {
	return generic.ElemTypeOf(v.ptr).String() == "bool"
}
