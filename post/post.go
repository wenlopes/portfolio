// Package post holds the Blog domain: a Post and its Versões, one per Idioma.
package post

import (
	"context"
	"errors"
	"time"
)

// Lang is the Idioma of a Versão.
type Lang string

// Supported Idiomas.
const (
	LangPT Lang = "pt"
	LangEN Lang = "en"
)

// State is the lifecycle state of a Versão.
type State string

// Versão states.
const (
	StateDraft     State = "draft"
	StatePublished State = "published"
)

// Domain errors.
var (
	// ErrNotFound is returned when a Post does not exist.
	ErrNotFound = errors.New("post not found")
	// ErrAlreadyExists is returned when creating a Post whose id is taken.
	ErrAlreadyExists = errors.New("post already exists")
	// ErrPublishedWithoutDate is returned when a Publicada Versão has no publication date.
	ErrPublishedWithoutDate = errors.New("published version without publication date")
)

// Post is an authored text on the Blog, written in one or two Versões.
type Post struct {
	ID       string
	Views    int64
	Versions []Version
}

// Version is a Post written in one Idioma.
type Version struct {
	Lang        Lang
	Title       string
	Slug        string
	Body        string
	State       State
	PublishedAt time.Time
}

// PublishedVersion is a Publicada Versão together with the id of its Post.
type PublishedVersion struct {
	PostID  string
	Version Version
}

// Repository persists Posts and their Versões.
type Repository interface {
	// Create stores a new Post with its initial Versões.
	Create(ctx context.Context, newPost Post) error
	// PutVersion creates or replaces one Versão of an existing Post, or returns ErrNotFound.
	PutVersion(ctx context.Context, in PutVersionInput) error
	// Get returns a Post with all its Versões, or ErrNotFound.
	Get(ctx context.Context, id string) (Post, error)
	// IncrementViews adds one Visualização to an existing Post and returns the new total, or ErrNotFound.
	IncrementViews(ctx context.Context, id string) (int64, error)
	// ListPublished returns the Publicada Versões of one Idioma, most recently published first.
	ListPublished(ctx context.Context, lang Lang) ([]PublishedVersion, error)
}

// PutVersionInput identifies the Versão written by Repository.PutVersion.
type PutVersionInput struct {
	PostID  string
	Version Version
}
