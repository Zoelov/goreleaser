// Package publish contains the publishing pipe.
package dockerpublish

import (
	"fmt"

	"github.com/goreleaser/goreleaser/internal/middleware"
	"github.com/goreleaser/goreleaser/internal/pipe/docker"
	"github.com/goreleaser/goreleaser/pkg/context"
	"github.com/pkg/errors"
)

// Pipe that publishes artifacts
type Pipe struct{}

func (Pipe) String() string {
	return "publishing docker"
}

// Publisher should be implemented by pipes that want to publish artifacts
type Publisher interface {
	fmt.Stringer

	// Default sets the configuration defaults
	Publish(ctx *context.Context) error
}

// nolint: gochecknoglobals
var publishers = []Publisher{
	docker.Pipe{},
}

// Run the pipe
func (Pipe) Run(ctx *context.Context) error {
	if ctx.Snapshot {
		for _, publisher := range publishers {
			if err := middleware.Logging(
				publisher.String(),
				middleware.ErrHandler(publisher.Publish),
				middleware.ExtraPadding,
			)(ctx); err != nil {
				return errors.Wrapf(err, "%s: failed to publish artifacts", publisher.String())
			}
		}
	}
	return nil
}
