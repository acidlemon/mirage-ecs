S3_BUCKET=$(shell tfstate-lookup aws_s3_bucket.mirage-ecs.bucket)
SHELL=/bin/bash

deploy/config:
	aws s3 cp config.yaml s3://$(S3_BUCKET)/config.yaml

deploy: deploy/config
	ecspresso verify
	ecspresso deploy

diff:
	diff -u <(aws s3 cp s3://$(S3_BUCKET)/config.yaml -) config.yaml || true
	ecspresso diff
