S3_BUCKET=$(shell tfstate-lookup aws_s3_bucket.mirage-ecs.bucket)
ECR_REPO=$(shell tfstate-lookup aws_ecr_repository.mirage-ecs.repository_url)
TAG?=$(shell git rev-parse --short HEAD)
SHELL=/bin/bash

deploy/config:
	aws s3 cp config.yaml s3://$(S3_BUCKET)/config.yaml

deploy: deploy/config
	ecspresso verify
	ecspresso deploy

diff:
	diff -u <(aws s3 cp s3://$(S3_BUCKET)/config.yaml -) config.yaml || true
	ecspresso diff

build-image:
	docker build -t $(ECR_REPO):$(TAG) -f ../docker/Dockerfile ..

push-image: build-image
	aws ecr get-login-password | docker login --username AWS --password-stdin $(firstword $(subst /, ,$(ECR_REPO)))
	docker push $(ECR_REPO):$(TAG)

# Build the local working tree, push it to ECR, and deploy it.
deploy/image: push-image deploy/config
	IMAGE=$(ECR_REPO):$(TAG) ecspresso verify
	IMAGE=$(ECR_REPO):$(TAG) ecspresso deploy
