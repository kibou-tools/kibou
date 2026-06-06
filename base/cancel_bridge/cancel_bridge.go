// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package cancel_bridge is used for hiding a cancel.Token
// inside a [context.Context].
//
// Generally, this is needed when using third-party APIs involving
// callbacks, where one registers callbacks of type func(context.Context, SomeArgs) error
// and primary API is invoked with func(context.Context, OtherArgs) error.
package cancel_bridge

import (
	"context" //nolint:depguard // cancel_bridge is the designated context-interop wrapper

	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/cancel"
)

type tokenKey struct{}

// Pre-condition: tok is non-nil.
func Inject(ctx context.Context, tok cancel.Token) context.Context {
	assert.Precondition(tok != nil, "tok argument must be non-nil")
	return context.WithValue(ctx, tokenKey{}, tok)
}

// Pre-condition: The argument ctx's construction should've used [cancel_bridge.Inject].
func Extract(ctx context.Context) cancel.Token {
	tok, ok := ctx.Value(tokenKey{}).(cancel.Token)
	assert.Preconditionf(ok, "no cancel.Token present on context; cancel_bridge.Inject was not called")
	return tok
}
