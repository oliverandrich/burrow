// Package jobs provides an in-process, SQLite-backed background job queue
// as a burrow contrib app. Jobs are enqueued with a type name and JSON payload,
// then processed by registered handler functions in a configurable worker pool.
package jobs
