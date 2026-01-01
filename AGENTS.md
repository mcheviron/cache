# Agent Guidelines for `cache-go/`

This directory is the Go implementation of the cache.

- Module: `github.com/mcheviron/cache`
- Go version: 1.22+

## Commands (use `Justfile`)

- List tasks: `just help`
- Install deps: `just install`
- Format: `just fmt`
- Lint: `just check` (runs `go vet`)
- Test all: `just test`

### Run a single test

- By name/regex: `just test-one TestCacheEvictsLhdWhenOverWeight`
- With `go test` directly: `go test ./... -run "TestCacheEvictsLhdWhenOverWeight"`

## Code layout

- `cache.go`: main cache API + eviction
- `config.go`: `Config` validation/build
- `item.go`: per-item TTL + atomic metadata
- `shard.go`: sharded map storage
- `*_test.go`: unit tests

## Style

### Formatting & imports

- Run `just fmt` (gofmt) before finalizing.
- No manual alignment; gofmt wins.
- Keep imports minimal and sorted; remove unused.

### Types & generics

- Public API should remain stable; keep parity with Rust/Zig.
- Avoid interface{} in the hot path unless unavoidable.

### Error handling

- Avoid panics in library code.
- For new fallible APIs, return `(T, error)` or `error` as appropriate.

### Concurrency

- Avoid holding shard locks across expensive work.
- Avoid calling user-provided `Weigher` under shard locks.
- Per-item access/hit metadata should be atomic.

## Testing philosophy (TDD)

This repo is **test-driven**:

- Every feature/change must come with tests that validate the behavior and the hypothesis behind the change.
- Prefer writing a failing test first, then implementing the smallest change to pass.

## Tests

- Add tests for behavior changes (TTL/eviction/weight semantics).
- Prefer deterministic tests; keep time-based waits bounded.
