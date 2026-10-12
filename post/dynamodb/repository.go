// Package dynamodb implements post.Repository on a single DynamoDB table.
package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/wenlopes/portfolio/post"
)

// Ensures Repository implements post.Repository.
var _ post.Repository = (*Repository)(nil)

const (
	postPrefix    = "POST#"
	metaSK        = "META"
	gsi1          = "GSI1"
	versionPrefix = "VERSION#"
)

// NewRepository creates a Repository backed by the given table.
func NewRepository(client *dynamodb.Client, tableName string) *Repository {
	return &Repository{client: client, tableName: tableName}
}

// Repository stores Posts as one META item plus one item per Versão.
type Repository struct {
	client    *dynamodb.Client
	tableName string
}

// Create stores the Post META item and its Versões atomically. It fails if the Post already exists.
func (r *Repository) Create(ctx context.Context, newPost post.Post) error {
	meta, err := attributevalue.MarshalMap(metaItem{PK: postPK(newPost.ID), SK: metaSK, Views: newPost.Views})
	if err != nil {
		return fmt.Errorf("marshal post meta: %w", err)
	}

	writes := []types.TransactWriteItem{{
		Put: &types.Put{
			TableName:           aws.String(r.tableName),
			Item:                meta,
			ConditionExpression: aws.String("attribute_not_exists(PK)"),
		},
	}}
	for _, version := range newPost.Versions {
		item, err := marshalVersion(newPost.ID, version)
		if err != nil {
			return err
		}
		writes = append(writes, types.TransactWriteItem{
			Put: &types.Put{TableName: aws.String(r.tableName), Item: item},
		})
	}

	_, err = r.client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: writes})
	if metaConditionFailed(err) {
		return fmt.Errorf("create post %s: %w", newPost.ID, post.ErrAlreadyExists)
	}
	if err != nil {
		return fmt.Errorf("create post %s: %w", newPost.ID, err)
	}

	return nil
}

// PutVersion creates or replaces one Versão, guarded by a check that the Post exists.
func (r *Repository) PutVersion(ctx context.Context, in post.PutVersionInput) error {
	item, err := marshalVersion(in.PostID, in.Version)
	if err != nil {
		return err
	}

	_, err = r.client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{
			{
				ConditionCheck: &types.ConditionCheck{
					TableName:           aws.String(r.tableName),
					Key:                 metaKey(in.PostID),
					ConditionExpression: aws.String("attribute_exists(PK)"),
				},
			},
			{Put: &types.Put{TableName: aws.String(r.tableName), Item: item}},
		},
	})
	if metaConditionFailed(err) {
		return fmt.Errorf("put version %s of post %s: %w", in.Version.Lang, in.PostID, post.ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("put version %s of post %s: %w", in.Version.Lang, in.PostID, err)
	}

	return nil
}

// Get returns the Post with all its Versões, or post.ErrNotFound.
func (r *Repository) Get(ctx context.Context, id string) (post.Post, error) {
	out, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: postPK(id)},
		},
	})
	if err != nil {
		return post.Post{}, fmt.Errorf("query post %s: %w", id, err)
	}

	result := post.Post{ID: id}
	found := false
	for _, raw := range out.Items {
		var key keyItem
		if err := attributevalue.UnmarshalMap(raw, &key); err != nil {
			return post.Post{}, fmt.Errorf("unmarshal post key: %w", err)
		}

		if key.SK == metaSK {
			var meta metaItem
			if err := attributevalue.UnmarshalMap(raw, &meta); err != nil {
				return post.Post{}, fmt.Errorf("unmarshal post meta: %w", err)
			}
			result.Views = meta.Views
			found = true
			continue
		}

		_, version, err := unmarshalVersion(raw)
		if err != nil {
			return post.Post{}, err
		}
		result.Versions = append(result.Versions, version)
	}

	if !found {
		return post.Post{}, fmt.Errorf("get post %s: %w", id, post.ErrNotFound)
	}

	return result, nil
}

// IncrementViews atomically adds one to the META item's counter, guarded by a check that the Post exists.
func (r *Repository) IncrementViews(ctx context.Context, id string) (int64, error) {
	out, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:           aws.String(r.tableName),
		Key:                 metaKey(id),
		UpdateExpression:    aws.String("ADD #views :one"),
		ConditionExpression: aws.String("attribute_exists(PK)"),
		ExpressionAttributeNames: map[string]string{
			"#views": "views",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":one": &types.AttributeValueMemberN{Value: "1"},
		},
		ReturnValues: types.ReturnValueUpdatedNew,
	})
	var conditionFailed *types.ConditionalCheckFailedException
	if errors.As(err, &conditionFailed) {
		return 0, fmt.Errorf("increment views of post %s: %w", id, post.ErrNotFound)
	}
	if err != nil {
		return 0, fmt.Errorf("increment views of post %s: %w", id, err)
	}

	var updated struct {
		Views int64 `dynamodbav:"views"`
	}
	if err := attributevalue.UnmarshalMap(out.Attributes, &updated); err != nil {
		return 0, fmt.Errorf("unmarshal views of post %s: %w", id, err)
	}

	return updated.Views, nil
}

// ListPublished queries GSI1, which only holds Publicada Versões, newest first.
func (r *Repository) ListPublished(ctx context.Context, lang post.Lang) ([]post.PublishedVersion, error) {
	paginator := dynamodb.NewQueryPaginator(r.client, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String(gsi1),
		KeyConditionExpression: aws.String("GSI1PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: publishedPK(lang)},
		},
		ScanIndexForward: aws.Bool(false),
	})

	var published []post.PublishedVersion
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("query published %s: %w", lang, err)
		}

		for _, raw := range page.Items {
			postID, version, err := unmarshalVersion(raw)
			if err != nil {
				return nil, err
			}
			published = append(published, post.PublishedVersion{PostID: postID, Version: version})
		}
	}

	return published, nil
}

type keyItem struct {
	PK string
	SK string
}

type metaItem struct {
	PK    string
	SK    string
	Views int64 `dynamodbav:"views"`
}

type versionItem struct {
	PK          string
	SK          string
	GSI1PK      string `dynamodbav:",omitempty"`
	GSI1SK      string `dynamodbav:",omitempty"`
	Lang        string `dynamodbav:"lang"`
	Title       string `dynamodbav:"title"`
	Slug        string `dynamodbav:"slug"`
	Body        string `dynamodbav:"body"`
	State       string `dynamodbav:"state"`
	PublishedAt string `dynamodbav:"publishedAt,omitempty"`
}

// unmarshalVersion decodes a Versão item and returns it with the id of its Post.
func unmarshalVersion(raw map[string]types.AttributeValue) (string, post.Version, error) {
	var item versionItem
	if err := attributevalue.UnmarshalMap(raw, &item); err != nil {
		return "", post.Version{}, fmt.Errorf("unmarshal version: %w", err)
	}

	version, err := item.toDomain()
	if err != nil {
		return "", post.Version{}, err
	}

	return strings.TrimPrefix(item.PK, postPrefix), version, nil
}

func (v versionItem) toDomain() (post.Version, error) {
	version := post.Version{
		Lang:  post.Lang(v.Lang),
		Title: v.Title,
		Slug:  v.Slug,
		Body:  v.Body,
		State: post.State(v.State),
	}
	if v.PublishedAt != "" {
		publishedAt, err := time.Parse(time.RFC3339Nano, v.PublishedAt)
		if err != nil {
			return post.Version{}, fmt.Errorf("parse publishedAt of %s: %w", v.SK, err)
		}
		version.PublishedAt = publishedAt
	}

	return version, nil
}

func marshalVersion(id string, version post.Version) (map[string]types.AttributeValue, error) {
	// GSI1 is sorted by publishedAt; without it the Versão would silently drop out of the index.
	if version.State == post.StatePublished && version.PublishedAt.IsZero() {
		return nil, fmt.Errorf("marshal version %s of post %s: %w", version.Lang, id, post.ErrPublishedWithoutDate)
	}

	item := versionItem{
		PK:    postPK(id),
		SK:    versionPrefix + string(version.Lang),
		Lang:  string(version.Lang),
		Title: version.Title,
		Slug:  version.Slug,
		Body:  version.Body,
		State: string(version.State),
	}
	if !version.PublishedAt.IsZero() {
		item.PublishedAt = version.PublishedAt.UTC().Format(time.RFC3339Nano)
	}
	if version.State == post.StatePublished {
		item.GSI1PK = publishedPK(version.Lang)
		item.GSI1SK = item.PublishedAt
	}

	attributes, err := attributevalue.MarshalMap(item)
	if err != nil {
		return nil, fmt.Errorf("marshal version %s: %w", version.Lang, err)
	}

	return attributes, nil
}

// metaConditionFailed reports whether a transaction was canceled by the META item's condition, always the first write.
func metaConditionFailed(err error) bool {
	var canceled *types.TransactionCanceledException
	if !errors.As(err, &canceled) || len(canceled.CancellationReasons) == 0 {
		return false
	}

	return aws.ToString(canceled.CancellationReasons[0].Code) == "ConditionalCheckFailed"
}

func metaKey(id string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: postPK(id)},
		"SK": &types.AttributeValueMemberS{Value: metaSK},
	}
}

func postPK(id string) string { return postPrefix + id }

func publishedPK(lang post.Lang) string {
	return "LANG#" + string(lang) + "#PUBLISHED"
}
