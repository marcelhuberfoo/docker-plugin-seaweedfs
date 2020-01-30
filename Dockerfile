FROM golang:1.13-alpine as builder

WORKDIR /src
COPY go.* /src/

RUN set -ex \
    && go mod download

ARG RELEASE_DATE
ENV RELEASE_DATE=$RELEASE_DATE
ARG COMMIT_HASH
ENV COMMIT_HASH=$COMMIT_HASH
ARG DIRTY
ENV DIRTY=$DIRTY

COPY *.go /src/

RUN set -ex \
    && go install --ldflags "-extldflags '-static' -X main.Version=${RELEASE_DATE} -X main.CommitHash=${COMMIT_HASH}${DIRTY}"
CMD ["/go/bin/docker-plugin-seaweedfs"]

FROM alpine:latest
####
# Install SeaweedFS Client
####
ARG SEAWEEDFS_VERSION=1.52
ENV SEAWEEDFS_VERSION=$SEAWEEDFS_VERSION
ARG PLUGIN_IMAGE_ROOTFS_TAG
ENV PLUGIN_IMAGE_ROOTFS_TAG=$PLUGIN_IMAGE_ROOTFS_TAG
# I have a docker socket, and this may help me test
ARG DOCKER_VERSION=19.03.5
ENV DOCKER_VERSION=$DOCKER_VERSION

RUN apk upgrade --no-cache \
    && apk add --no-cache fuse \
    && apk add --no-cache --virtual build-dependencies ca-certificates tar \
    && wget --quiet -O - https://github.com/chrislusf/seaweedfs/releases/download/${SEAWEEDFS_VERSION}/linux_amd64.tar.gz | \
        tar xzvf - -C /usr/bin/ \
    && wget --quiet -O - https://download.docker.com/linux/static/stable/x86_64/docker-${DOCKER_VERSION}.tgz | \
        tar xzvf - -C /bin --strip-components=1 docker/docker \
    && apk del --no-cache build-dependencies

# let non-root users fusemount
RUN echo "user_allow_other" >> /etc/fuse.conf

RUN mkdir -p /run/docker/plugins /mnt/state /mnt/volumes

COPY --from=builder /go/bin/docker-plugin-seaweedfs .
CMD ["/docker-plugin-seaweedfs"]
