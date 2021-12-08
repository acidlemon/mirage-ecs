GIT_VER := $(shell git describe --tags)
DATE := $(shell date +%Y-%m-%dT%H:%M:%S%z)
export GO111MODULE := on

mirage-ecs: *.go
	CGO_ENABLED=0 go build --ldflags '-extldflags "-static"' -o mirage-ecs  .

clean:
	rm -rf dist/* mirage-ecs

run: mirage-ecs
	./mirage-ecs

packages:
	goreleaser release --rm-dist --snapshot --skip-publish

docker-image:
	docker build -t mirage-ecs -f docker/Dockerfile .

test:
	go test -v ./...
