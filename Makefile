GIT_VER := $(shell git describe --tags)
DATE := $(shell date +%Y-%m-%dT%H:%M:%S%z)
export GO111MODULE := on

mirage-ecs: *.go cmd/mirage-ecs/*.go go.mod go.sum
	CGO_ENABLED=0 go build -ldflags "-X main.Version=$(GIT_VER) -X main.buildDate=$(DATE)" -o mirage-ecs ./cmd/mirage-ecs/main.go

clean:
	rm -rf dist/* mirage-ecs

run: mirage-ecs
	./mirage-ecs

packages:
	goreleaser release --rm-dist --snapshot --skip-publish

docker-image:
	docker build -t ghcr.io/acidlemon/mirage-ecs:$(GIT_VER) -f docker/Dockerfile .

push-image: docker-image
	docker push ghcr.io/acidlemon/mirage-ecs:$(GIT_VER)

test:
	go test -v ./...

install:
	go install github.com/acidlemon/mirage-ecs/v2/cmd/mirage-ecs
