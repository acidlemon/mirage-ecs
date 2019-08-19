FROM golang:1.12 AS builder

ADD . /stash/src/github.com/acidlemon/mirage-ecs
WORKDIR /stash/src/github.com/acidlemon/mirage-ecs
ENV GOPATH=/stash

RUN make && mv mirage-ecs /stash/
RUN cp -a html /stash/
RUN cp docker/config.yml /stash/config.yml

FROM alpine:3.9
MAINTAINER acidlemon

RUN apk --no-cache add ca-certificates
RUN mkdir -p /opt/mirage/html
COPY --from=builder /stash/mirage-ecs /opt/mirage/
COPY --from=builder /stash/config.yml /opt/mirage/
COPY --from=builder /stash/html/* /opt/mirage/html/
WORKDIR /opt/mirage
ENV MIRAGE_LOG_LEVEL info
ENV MIRAGE_CONF config.yml

ENTRYPOINT ["/opt/mirage/mirage-ecs"]
