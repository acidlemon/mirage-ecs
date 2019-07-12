GIT_VER := $(shell git describe --tags)
DATE := $(shell date +%Y-%m-%dT%H:%M:%S%z)
export GO111MODULE := on

mirage: *.go
	go build -o mirage .

clean:
	rm -rf pkg/* mirage

run: mirage
	./mirage -conf config_sample.yml

binary: clean
	CGO_ENABLED=0 gox -osarch="linux/amd64 darwin/amd64 windows/amd64 windows/386" -output "pkg/{{.Dir}}-${GIT_VER}-{{.OS}}-{{.Arch}}" -ldflags "-X main.version=${GIT_VER} -X main.buildDate=${DATE} -extldflags \"-static\""

package: binary
	cd ./pkg && find . -name "*${GIT_VER}*" -type f \
         -exec mkdir -p mirage/data \;  \
         -exec cp {} mirage/mirage \;   \
         -exec cp -r ../html ../config_sample.yml mirage/ \; \
         -exec zip -r {}.zip mirage \;     \
         -exec rm -rf mirage \;

