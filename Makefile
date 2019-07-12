GIT_VER := $(shell git describe --tags)
DATE := $(shell date +%Y-%m-%dT%H:%M:%S%z)
export GO111MODULE := on

mirage-ecs: *.go
	go build -o mirage-ecs .

clean:
	rm -rf pkg/* mirage-ecs

run: mirage-ecs
	./mirage-ecs
