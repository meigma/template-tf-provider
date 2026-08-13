// Package client implements the core.Store port against a JSON file on disk.
//
// It is an adapter, and a deliberately narrow one: it stores items and does
// nothing else. A real provider would talk to an HTTP API here, and the shape
// would be the same — a constructor that takes connection details, a type that
// satisfies the port, and no knowledge of Terraform anywhere in the package.
//
// A file is used instead of an in-memory map because Terraform starts a fresh
// provider process for every command. State kept in memory would not survive
// the gap between plan, apply, and destroy.
//
// # Concurrency
//
// A [Store] serializes its own operations, so concurrent use within one
// process is safe. It takes no file lock, so two processes writing the same
// file can still lose an update: the last writer's copy of the document wins.
// That is an acceptable limit for an example provider, and calling it out here
// is cheaper than the cross-platform locking it would take to fix.
package client
