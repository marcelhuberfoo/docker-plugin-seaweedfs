PREFIX = svendowideit/seaweedfs-volume
PLUGIN_NAME = ${PREFIX}-plugin
PLUGIN_TAG ?= next

RELEASE_DATE=$(shell date +%F)
COMMIT_HASH=$(shell git rev-parse --short HEAD 2>/dev/null)
GITSTATUS=$(shell git status --porcelain --untracked-files=no)
ifneq ($(GITSTATUS),)
  DIRTY=-dirty
endif

all: clean rootfs create enable

build:
	go build --ldflags "-extldflags '-static' -X main.Version=${RELEASE_DATE} -X main.CommitHash=${COMMIT_HASH}${DIRTY}" .

clean:
	@echo "### rm ./plugin"
	@rm -rf ./plugin

rootfs:
	@echo "### docker build: rootfs image with ${PLUGIN_NAME}-rootfs (${RELEASE_DATE}) ${COMMIT_HASH}${DIRTY}"
	@echo "${GITSTATUS}"
	@docker build --target builder -t ${PLUGIN_NAME}-rootfs:build-${PLUGIN_TAG} --build-arg "RELEASE_DATE=${RELEASE_DATE}" --build-arg "COMMIT_HASH=${COMMIT_HASH}" --build-arg "DIRTY=${DIRTY}" .
	@docker build -t ${PLUGIN_NAME}-rootfs:${PLUGIN_TAG} --build-arg "RELEASE_DATE=${RELEASE_DATE}" --build-arg "COMMIT_HASH=${COMMIT_HASH}" --build-arg "DIRTY=${DIRTY}" .
	@echo "### create rootfs directory in ./plugin/rootfs"
	@mkdir -p ./plugin/rootfs
	@docker create --name tmp ${PLUGIN_NAME}-rootfs:${PLUGIN_TAG}
	@docker export tmp | tar -x -C ./plugin/rootfs
	@echo "### add version into to config.json and stage into ./plugin/"
	@RELEASE_DATE=${RELEASE_DATE} COMMIT_HASH=${COMMIT_HASH} DIRTY=${DIRTY} envsubst > ./plugin/config.json < config.json
	@docker rm -vf tmp

create:
	@echo "### remove existing plugin swarm if exists"
	@docker volume rm -f test || true
	@docker plugin rm -f swarm || true
	@echo "### create new plugin swarm from ./plugin"
	@docker plugin create swarm ./plugin
	@docker plugin set swarm DEBUG=true
	@echo "### create new plugin for pushing to Docker hub ${PLUGIN_NAME}:${PLUGIN_TAG} from ./plugin"
	@docker plugin rm -f ${PLUGIN_NAME}:${PLUGIN_TAG} || true
	@docker plugin create ${PLUGIN_NAME}:${PLUGIN_TAG} ./plugin



enable:		
	@echo "### enable plugin swarm"		
	@docker plugin enable swarm

ps:
	@ps -U root -u | grep docker-plugin-seaweedf

enter:
	@sudo nsenter --target $(shell ps -U root -u | grep docker-plugin-seaweedf | xargs | cut -f2 -d" ") --mount --uts --ipc --net --pid sh

sven:
	@docker kill tester | true
	@docker volume rm -f test3 | true
	@sleep 1

	@docker volume create -d swarm -o uid=33 -o gid=10 -o umask=0773 test3
	@docker run -d --name tester -u 33  --rm -it -v test3:/test debian sh

	@docker run --rm -it -v test3:/test debian ls -al | grep test
	@docker run --rm -it -v test3:/test debian bash -c "mktemp -p /test/"
	@docker run --rm -it -u 33 -v test3:/test debian bash -c "mktemp -p /test/"
	@docker run --rm -it -v test3:/test debian ls -al /test/

	@echo "is the volume plugin mount container running:"
	@docker ps | grep seaweed-volume | grep test3

	@docker kill tester

	@echo "is the volume plugin mount container gone:"
	@docker ps -a | grep seaweed-volume-proxy | grep test3 | true

	@docker volume rm -f test3 | true


mountall:
	@docker run --rm -it --net=seaweedfs_internal --cap-add=SYS_ADMIN --device=/dev/fuse:/dev/fuse --security-opt=apparmor:unconfined --entrypoint=weed svendowideit/seaweedfs-volume-plugin-rootfs:next mount -filer=filer:8888 -dir=/mnt -filer.path=/


logs:
	@sudo journalctl -fu docker | grep seaweedfs

push:  clean rootfs create enable
	@echo "### push plugin ${PLUGIN_NAME}:${PLUGIN_TAG}"
	@docker push ${PLUGIN_NAME}-rootfs:build-${PLUGIN_TAG}
	@docker push ${PLUGIN_NAME}-rootfs:${PLUGIN_TAG}
	@docker plugin push ${PLUGIN_NAME}:${PLUGIN_TAG}
