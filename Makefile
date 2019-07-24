PLUGIN_NAME = svendowideit/docker-volume-seaweedfs
PLUGIN_TAG ?= next

all: clean rootfs create enable

clean:
	@echo "### rm ./plugin"
	@rm -rf ./plugin

rootfs:
	@echo "### docker build: rootfs image with ${PLUGIN_NAME}"
	@docker build --target builder -t ${PLUGIN_NAME}:build .

	@docker build -t ${PLUGIN_NAME}:rootfs .
	@echo "### create rootfs directory in ./plugin/rootfs"
	@mkdir -p ./plugin/rootfs
	@docker create --name tmp ${PLUGIN_NAME}:rootfs
	@docker export tmp | tar -x -C ./plugin/rootfs
	@echo "### copy config.json to ./plugin/"
	@cp config.json ./plugin/
	@docker rm -vf tmp

create:
	@echo "### remove existing plugin ${PLUGIN_NAME}:${PLUGIN_TAG} if exists"
	@docker volume rm -f test || true
	@docker plugin rm -f ${PLUGIN_NAME}:${PLUGIN_TAG} || true
	@echo "### create new plugin ${PLUGIN_NAME}:${PLUGIN_TAG} from ./plugin"
	@docker plugin create ${PLUGIN_NAME}:${PLUGIN_TAG} ./plugin
	@docker plugin set ${PLUGIN_NAME}:${PLUGIN_TAG} DEBUG=true

enable:		
	@echo "### enable plugin ${PLUGIN_NAME}:${PLUGIN_TAG}"		
	@docker plugin enable ${PLUGIN_NAME}:${PLUGIN_TAG}

ps:
	ps -U root -u | grep docker-plugin-seaweedf

enter:
	sudo nsenter --target $(shell ps -U root -u | grep docker-plugin-seaweedf | xargs | cut -f2 -d" ") --mount --uts --ipc --net --pid sh

sven:
	docker volume create -d $(shell docker plugin ls --format={{.ID}}) test
	docker run --rm -it -v test:/test debian

logs:
	sudo journalctl -fu docker | grep seaweedfs

push:  clean rootfs create enable
	@echo "### push plugin ${PLUGIN_NAME}:${PLUGIN_TAG}"
	@docker plugin push ${PLUGIN_NAME}:rootfs
	@docker plugin push ${PLUGIN_NAME}:${PLUGIN_TAG}
