// Package client contains the client implementations for several providers.
package client

import (
	"os"
	"time"

	"github.com/goreleaser/goreleaser/internal/artifact"
	"github.com/goreleaser/goreleaser/pkg/config"
	"github.com/goreleaser/goreleaser/pkg/context"
)

// Info of the repository
type Info struct {
	Description string
	Homepage    string
	URL         string
}

// CommitInfo commit information
type CommitInfo struct {
	ID             string `json:"id"`
	ShortID        string `json:"short_id"`
	CreatedAt      *time.Time
	ParentIds      []string
	Title          string
	Message        string
	AuthorName     string
	AuthorEmail    string
	AuthoredDate   *time.Time
	CommitterName  string
	CommitterEmail string
	CommittedDate  *time.Time
	Status         string
	BaseURL        string
	AvatarURL      string
}

// Client interface
type Client interface {
	CreateRelease(ctx *context.Context, body string) (releaseID string, err error)
	CreateFile(ctx *context.Context, commitAuthor config.CommitAuthor, repo config.Repo, content []byte, path, message string) (err error)
	Upload(ctx *context.Context, releaseID string, artifact *artifact.Artifact, file *os.File) (err error)
	GetInfoByID(ctx *context.Context, commitID string) (*CommitInfo, error)
}

// New creates a new client depending on the token type
func New(ctx *context.Context) (Client, error) {
	if ctx.TokenType == context.TokenTypeGitHub {
		return NewGitHub(ctx)
	}
	if ctx.TokenType == context.TokenTypeGitLab {
		return NewGitLab(ctx)
	}
	if ctx.TokenType == context.TokenTypeGitea {
		return NewGitea(ctx)
	}
	return nil, nil
}
