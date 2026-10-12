//go:build integration

package dynamodb_test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	internaldynamodb "github.com/wenlopes/portfolio/internal/dynamodb"
	"github.com/wenlopes/portfolio/post"
	postdynamodb "github.com/wenlopes/portfolio/post/dynamodb"
)

func newRepository(t *testing.T) *postdynamodb.Repository {
	t.Helper()

	tableName := os.Getenv("TABLE_NAME")
	require.NotEmpty(t, tableName, "TABLE_NAME must point to the portfolio-data table (run make deploy-local)")

	client, err := internaldynamodb.NewClient(t.Context())
	require.NoError(t, err)

	return postdynamodb.NewRepository(client, tableName)
}

func uniqueID(t *testing.T) string {
	t.Helper()
	return t.Name() + "-" + time.Now().Format("150405.000000000")
}

func TestRepository_CreateAndGet(t *testing.T) {
	t.Parallel()

	// Arrange.
	repository := newRepository(t)
	id := uniqueID(t)
	draft := post.Version{
		Lang:  post.LangPT,
		Title: "Idempotência em pagamentos",
		Slug:  "idempotencia-em-pagamentos",
		Body:  "# Idempotência\n\nCorpo em Markdown.",
		State: post.StateDraft,
	}

	// Act.
	err := repository.Create(t.Context(), post.Post{ID: id, Versions: []post.Version{draft}})
	require.NoError(t, err)
	got, err := repository.Get(t.Context(), id)

	// Assert.
	require.NoError(t, err)
	assert.Equal(t, post.Post{ID: id, Views: 0, Versions: []post.Version{draft}}, got)
}

func TestRepository_GetUnknownPost(t *testing.T) {
	t.Parallel()

	// Arrange.
	repository := newRepository(t)

	// Act.
	_, err := repository.Get(t.Context(), uniqueID(t))

	// Assert.
	assert.ErrorIs(t, err, post.ErrNotFound)
}

func TestRepository_CreateExistingPost(t *testing.T) {
	t.Parallel()

	// Arrange.
	repository := newRepository(t)
	id := uniqueID(t)
	require.NoError(t, repository.Create(t.Context(), post.Post{ID: id}))

	// Act.
	err := repository.Create(t.Context(), post.Post{ID: id})

	// Assert.
	assert.ErrorIs(t, err, post.ErrAlreadyExists)
}

func TestRepository_PutVersion(t *testing.T) {
	t.Parallel()

	// Arrange.
	repository := newRepository(t)
	id := uniqueID(t)
	ptDraft := post.Version{Lang: post.LangPT, Title: "Rascunho", Slug: "rascunho", Body: "v1", State: post.StateDraft}
	require.NoError(t, repository.Create(t.Context(), post.Post{ID: id, Versions: []post.Version{ptDraft}}))
	ptEdited := post.Version{Lang: post.LangPT, Title: "Título final", Slug: "rascunho", Body: "v2", State: post.StateDraft}
	enDraft := post.Version{Lang: post.LangEN, Title: "Draft", Slug: "draft", Body: "en body", State: post.StateDraft}

	// Act.
	require.NoError(t, repository.PutVersion(t.Context(), post.PutVersionInput{PostID: id, Version: ptEdited}))
	require.NoError(t, repository.PutVersion(t.Context(), post.PutVersionInput{PostID: id, Version: enDraft}))
	got, err := repository.Get(t.Context(), id)

	// Assert.
	require.NoError(t, err)
	assert.ElementsMatch(t, []post.Version{ptEdited, enDraft}, got.Versions)
}

func TestRepository_PutVersionOnUnknownPost(t *testing.T) {
	t.Parallel()

	// Arrange.
	repository := newRepository(t)
	draft := post.Version{Lang: post.LangPT, Title: "Órfã", Slug: "orfa", State: post.StateDraft}

	// Act.
	err := repository.PutVersion(t.Context(), post.PutVersionInput{PostID: uniqueID(t), Version: draft})

	// Assert.
	assert.ErrorIs(t, err, post.ErrNotFound)
}

func TestRepository_ListPublished(t *testing.T) {
	t.Parallel()

	// Arrange.
	repository := newRepository(t)
	older := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	olderPT := post.Version{Lang: post.LangPT, Title: "Antigo", Slug: "antigo", State: post.StatePublished, PublishedAt: older}
	newerPT := post.Version{Lang: post.LangPT, Title: "Novo", Slug: "novo", State: post.StatePublished, PublishedAt: newer}
	draftPT := post.Version{Lang: post.LangPT, Title: "Rascunho", Slug: "rascunho", State: post.StateDraft}
	publishedEN := post.Version{Lang: post.LangEN, Title: "English", Slug: "english", State: post.StatePublished, PublishedAt: newer}
	unpublishedPT := post.Version{Lang: post.LangPT, Title: "Despublicado", Slug: "despublicado", State: post.StatePublished, PublishedAt: newer}

	olderID, newerID, draftID, unpublishedID := uniqueID(t)+"-older", uniqueID(t)+"-newer", uniqueID(t)+"-draft", uniqueID(t)+"-unpublished"
	require.NoError(t, repository.Create(t.Context(), post.Post{ID: olderID, Versions: []post.Version{olderPT}}))
	require.NoError(t, repository.Create(t.Context(), post.Post{ID: newerID, Versions: []post.Version{newerPT, publishedEN}}))
	require.NoError(t, repository.Create(t.Context(), post.Post{ID: draftID, Versions: []post.Version{draftPT}}))
	require.NoError(t, repository.Create(t.Context(), post.Post{ID: unpublishedID, Versions: []post.Version{unpublishedPT}}))
	unpublishedPT.State, unpublishedPT.PublishedAt = post.StateDraft, time.Time{}
	require.NoError(t, repository.PutVersion(t.Context(), post.PutVersionInput{PostID: unpublishedID, Version: unpublishedPT}))

	// Act.
	got, err := repository.ListPublished(t.Context(), post.LangPT)

	// Assert.
	require.NoError(t, err)
	// The table is shared with other tests, so only this test's Posts are compared.
	mine := map[string]bool{olderID: true, newerID: true, draftID: true, unpublishedID: true}
	var ours []post.PublishedVersion
	for _, published := range got {
		if mine[published.PostID] {
			ours = append(ours, published)
		}
	}
	assert.Equal(t, []post.PublishedVersion{
		{PostID: newerID, Version: newerPT},
		{PostID: olderID, Version: olderPT},
	}, ours)
}

func TestRepository_IncrementViews(t *testing.T) {
	t.Parallel()

	// Arrange.
	repository := newRepository(t)
	id := uniqueID(t)
	require.NoError(t, repository.Create(t.Context(), post.Post{ID: id}))

	// Act.
	first, err := repository.IncrementViews(t.Context(), id)
	require.NoError(t, err)
	second, err := repository.IncrementViews(t.Context(), id)
	require.NoError(t, err)
	got, err := repository.Get(t.Context(), id)

	// Assert.
	require.NoError(t, err)
	assert.Equal(t, int64(1), first)
	assert.Equal(t, int64(2), second)
	assert.Equal(t, int64(2), got.Views)
}

func TestRepository_IncrementViewsOnUnknownPost(t *testing.T) {
	t.Parallel()

	// Arrange.
	repository := newRepository(t)
	id := uniqueID(t)

	// Act.
	_, err := repository.IncrementViews(t.Context(), id)

	// Assert.
	require.ErrorIs(t, err, post.ErrNotFound)
	_, err = repository.Get(t.Context(), id)
	assert.ErrorIs(t, err, post.ErrNotFound, "counting a view must not create a Post")
}

func TestRepository_PublishedVersionWithoutDate(t *testing.T) {
	t.Parallel()

	// Arrange.
	repository := newRepository(t)
	id := uniqueID(t)
	require.NoError(t, repository.Create(t.Context(), post.Post{ID: id}))
	undated := post.Version{Lang: post.LangPT, Title: "Sem data", Slug: "sem-data", State: post.StatePublished}

	// Act.
	err := repository.PutVersion(t.Context(), post.PutVersionInput{PostID: id, Version: undated})

	// Assert.
	assert.ErrorIs(t, err, post.ErrPublishedWithoutDate)
}
