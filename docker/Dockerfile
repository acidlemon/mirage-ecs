FROM scratch
MAINTAINER acidlemon

RUN mkdir /opt/mirage
COPY mirage-ecs config.yml /opt/mirage/
WORKDIR /opt/mirage
ENV MIRAGE_LOG_LEVEL info
ENV MIRAGE_CONF config.yml
CMD ["./mirage"]
