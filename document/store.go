// Package document defines the optional boundary for storing complete D2 sources.
// The shipped server does not configure a Store or provide a persistence backend.
package document

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("document not found")
var ErrConflict = errors.New("document revision conflict")

// Document is a complete source snapshot. Revisions are opaque, nonempty tokens.
// A revision must change after every successful write, including identical source.
type Document struct {
	ID       string `json:"id"`
	Revision string `json:"revision"`
	Code     string `json:"code"`
}

// Store implementations must enforce authorization from ctx, size/retention
// limits, and atomic compare-and-swap across ALL server replicas. IDs and
// revisions are references, not credentials. Never use an ID as a filesystem path.
//
// Get returns the current complete snapshot. Put creates only when expectedRevision
// is empty, otherwise replaces only the matching revision. Missing reads return
// ErrNotFound; failed preconditions return ErrConflict. A successful Put returns
// the committed snapshot. Implementations must be safe for concurrent calls and
// honor cancellation. There is intentionally no process-local default backend.
type Store interface {
	Get(ctx context.Context, id string) (Document, error)
	Put(ctx context.Context, id, expectedRevision, code string) (Document, error)
}
