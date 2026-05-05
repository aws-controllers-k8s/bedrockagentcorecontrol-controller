AWS_REGION ?= "us-west-2"
ECR_REPOSITORY ?= ack-e2e-testing-bedrockagentcorecontrol-controller
IMAGE_TAG ?= v1

AWS_ACCOUNT_ID ?= $(shell aws sts get-caller-identity --query "Account" --output text)
IMAGE_URL ?= $(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com/$(ECR_REPOSITORY):$(IMAGE_TAG)

public-ecr-auth:
	aws ecr-public get-login-password --region us-east-1 | \
		docker login --username AWS --password-stdin \
		public.ecr.aws

build-image: public-ecr-auth
	docker build --platform linux/amd64 --provenance=false -t $(IMAGE_URL) .

publish-image:
	docker push $(IMAGE_URL)

create-ecr-repository:
	aws ecr create-repository --region $(AWS_REGION) --repository-name $(ECR_REPOSITORY) >/dev/null

docker-ecr-auth:
	aws ecr get-login-password --region $(AWS_REGION) | \
		docker login --username AWS --password-stdin \
		$(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com

all: build-image publish-image
