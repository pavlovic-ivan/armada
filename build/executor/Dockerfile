ARG BASE_IMAGE=alpine:3.21.3

FROM ${BASE_IMAGE}
LABEL org.opencontainers.image.title=executor
LABEL org.opencontainers.image.description="Executor"
LABEL org.opencontainers.image.url=https://hub.docker.com/r/gresearch/executor

RUN addgroup -S -g 2000 armada && adduser -S -u 1000 armada -G armada
USER armada

COPY executor /app/
COPY config/executor/config.yaml /app/config/executor/config.yaml
COPY config/logging.yaml /app/config/logging.yaml

WORKDIR /app

ENTRYPOINT ["./executor"]
