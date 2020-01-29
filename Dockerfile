FROM golang:1.13-alpine as builder

ARG RELEASE_DATE
ENV RELEASE_DATE=$RELEASE_DATE
ARG COMMIT_HASH
ENV COMMIT_HASH=$COMMIT_HASH
ARG DIRTY
ENV DIRTY=$DIRTY

WORKDIR /src
COPY . /src/

RUN set -ex \
    && apk add --no-cache --virtual .build-deps gcc libc-dev git \
    && go mod download \
    && go install --ldflags "-extldflags '-static' -X main.Version=${RELEASE_DATE} -X main.CommitHash=${COMMIT_HASH}${DIRTY}" \
    && mkdir -p /app && cp -p $GOPATH/bin/docker-plugin-seaweedfs /app \
    && rm -rf $GOPATH \
    && apk del --no-cache .build-deps
CMD ["/app/docker-plugin-seaweedfs"]

FROM alpine:latest
####
# Install SeaweedFS Client
####
ARG SEAWEEDFS_VERSION=1.52
ENV SEAWEEDFS_VERSION=$SEAWEEDFS_VERSION
RUN apk upgrade --no-cache && \
    apk add --no-cache fuse && \
    apk add --no-cache --virtual build-dependencies ca-certificates && \
    wget -qO /tmp/linux_amd64.tar.gz https://github.com/chrislusf/seaweedfs/releases/download/${SEAWEEDFS_VERSION}/linux_amd64.tar.gz && \
    tar -C /usr/bin/ -xzvf /tmp/linux_amd64.tar.gz && \
    apk del --no-cache build-dependencies && \
    rm -rf /tmp/*

# I have a docker socket, and this may help me test
ARG DOCKER_VERSION=19.03.5
ENV DOCKER_VERSION=$DOCKER_VERSION
RUN cd /tmp \
    && wget --quiet https://download.docker.com/linux/static/stable/x86_64/docker-${DOCKER_VERSION}.tgz \
    && tar zxvf docker-${DOCKER_VERSION}.tgz \
    && cp docker/docker /bin/ \
    && rm -rf docker*

# let non-root users fusemount
RUN echo "user_allow_other" >> /etc/fuse.conf

RUN mkdir -p /run/docker/plugins /mnt/state /mnt/volumes

COPY --from=builder /app/docker-plugin-seaweedfs .
CMD ["/docker-plugin-seaweedfs"]
