FROM alpine:3.21

ARG TARGETARCH

RUN apk add --no-cache ca-certificates tzdata

COPY dist/${TARGETARCH}/registry-server-linux-${TARGETARCH} /usr/local/bin/registry-server

RUN chmod +x /usr/local/bin/registry-server

RUN mkdir -p /data

EXPOSE 9080

ENTRYPOINT ["registry-server"]
CMD ["--addr", ":9080", "--data-dir", "/data"]
