# Testing style guide

## Test code is also just code

Principles that apply to well-written production code
also apply to test code.

- Factor common things out into libraries.
- Pay attention to names and readability.
- Add comments for explaining rationale,
  tricky bits of code etc.

## Write tests in the same language as the code

An example of using a custom DSL for testing is in
[cmd/go/testdata/script](/go/src/cmd/go/testdata/script/README).
Writing tests like this means that:

1. You don't benefit from tooling already available
   for the host language (e.g. code navigation).
2. You have to learn the quirks of the DSL which
   do not help you in other contexts.

Instead, write the test using normal code as possible.
If there's a lot of boilerplate and repetition,
define helper types and methods as necessary.

It is perfectly fine to wrap standard types
(for example, `exec.Command`) into more
test-friendly types that make the test more readable.
