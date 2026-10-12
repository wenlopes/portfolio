# Portfolio Makefile.

STACK_PREFIX := portfolio
REGION       := us-east-1
LOCAL_BUCKET := portfolio-artifacts-local

# AWS CLI pointed at MiniStack, run from the official image so no local install is needed.
LOCAL_AWS := docker run --rm --network host \
	-e AWS_ACCESS_KEY_ID=test -e AWS_SECRET_ACCESS_KEY=test \
	-e AWS_DEFAULT_REGION=$(REGION) -e AWS_ENDPOINT_URL=http://localhost:4566 \
	-v "$(CURDIR)":/aws amazon/aws-cli

# Environment for Go code talking to MiniStack.
LOCAL_ENV := AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test \
	AWS_REGION=$(REGION) AWS_ENDPOINT_URL=http://localhost:4566

# Start MiniStack on port 4566.
up:
	@docker start ministack 2>/dev/null || docker run -d --name ministack -p 4566:4566 ministackorg/ministack
	@until curl -sf http://localhost:4566/_ministack/health > /dev/null; do sleep 1; done
	@echo "MiniStack up on http://localhost:4566"

# Stop and remove MiniStack (state is in memory and is lost).
down:
	@docker rm -f ministack

# Build the Lambda bootstrap for arm64 and zip it.
build-lambda:
	@mkdir -p build
	@GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags lambda.norpc -o build/bootstrap ./cmd/api
	@cd build && rm -f api.zip && zip -q api.zip bootstrap

# Deploy the data and app stacks to MiniStack.
deploy-local: build-lambda
	@$(LOCAL_AWS) cloudformation deploy --stack-name $(STACK_PREFIX)-data --template-file infra/data.yaml
	@$(LOCAL_AWS) s3api head-bucket --bucket $(LOCAL_BUCKET) 2>/dev/null || $(LOCAL_AWS) s3 mb s3://$(LOCAL_BUCKET)
	@$(LOCAL_AWS) cloudformation package --template-file infra/app.yaml \
		--s3-bucket $(LOCAL_BUCKET) --output-template-file build/app.packaged.yaml
	@$(LOCAL_AWS) cloudformation deploy --stack-name $(STACK_PREFIX)-app \
		--template-file build/app.packaged.yaml --capabilities CAPABILITY_IAM

# Unit tests.
test:
	@go test ./...

# Integration tests against the data stack on MiniStack (run make up deploy-local first).
test-integration:
	@$(LOCAL_ENV) TABLE_NAME=$$($(LOCAL_AWS) cloudformation describe-stacks --stack-name $(STACK_PREFIX)-data \
		--query "Stacks[0].Outputs[?OutputKey=='TableName'].OutputValue" --output text) \
		go test -tags integration ./...

tidy:
	@go mod tidy

lint:
	@golangci-lint run --build-tags integration ./...

.PHONY: up down build-lambda deploy-local test test-integration tidy lint
