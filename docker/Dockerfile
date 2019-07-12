FROM alpine:3.9
MAINTAINER acidlemon

RUN apk --no-cache add ca-certificates
RUN mkdir -p /opt/mirage/html
COPY mirage-ecs config.yml /opt/mirage/
COPY html/* /opt/mirage/html/
WORKDIR /opt/mirage
ENV MIRAGE_LOG_LEVEL info
ENV MIRAGE_CONF config.yml

ENTRYPOINT ["/opt/mirage/mirage-ecs"]
