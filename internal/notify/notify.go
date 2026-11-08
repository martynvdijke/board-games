// Package notify delivers optional, fire-and-forget notifications about
// application events. A nil Notifier means notifications are disabled.
package notify

import "context"

// Notifier sends a short notification. Implementations must return quickly;
// callers invoke them from a background goroutine.
type Notifier interface {
	Notify(ctx context.Context, title, body string) error
}
