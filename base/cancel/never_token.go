// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cancel

import (
	"context"

	. "code.kibou.tools/base/unit"
)

var neverTokenSingleton = neverToken{}

// Never returns a Token that is never canceled.
//
// Generally a replacement for [context.Background] from the standard library.
func Never() Token {
	return &neverTokenSingleton
}

// neverToken in a more lightweight implementation of Token
//
// Since this cannot be canceled, there's no notion of propagating
// the cancellation to a child, so we don't need to track children,
// unlike rawToken.
type neverToken struct{}

var _ Token = (*neverToken)(nil)

func (neverToken) KeepGoing() error {
	return nil
}

func (neverToken) Done() <-chan Unit {
	return nil
}

func (t neverToken) AsStdlibContext() context.Context {
	return tokenContext{tok: t}
}

func (neverToken) NewChild() ChildToken {
	return newChildToken(nil)
}
