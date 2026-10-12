// Package dynamodb provides the technical building blocks to talk to DynamoDB.
package dynamodb

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// NewClient builds a DynamoDB client from the default AWS configuration chain.
// AWS_ENDPOINT_URL, when set, points the client at MiniStack.
func NewClient(ctx context.Context) (*dynamodb.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	return dynamodb.NewFromConfig(cfg), nil
}
