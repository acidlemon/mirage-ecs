FROM golang:1.20 AS builder

ADD . /stash/src/github.com/acidlemon/mirage-ecs
WORKDIR /stash/src/github.com/acidlemon/mirage-ecs
ENV GOPATH=/stash

RUN make && mv mirage-ecs /stash/
RUN cp -a html /stash/
RUN cp docker/example-config.yml /stash/

FROM debian:bullseye-slim

RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
RUN mkdir -p /opt/mirage/html
COPY --from=builder /stash/mirage-ecs /opt/mirage/
COPY --from=builder /stash/example-config.yml /opt/mirage/
COPY --from=builder /stash/html/* /opt/mirage/html/
WORKDIR /opt/mirage
ENV MIRAGE_LOG_LEVEL info
ENV MIRAGE_CONF ""

ENTRYPOINT ["/opt/mirage/mirage-ecs"]
