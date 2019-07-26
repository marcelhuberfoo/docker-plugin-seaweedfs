# SeaweedFS Docker Plugin

This is stupendously experimental. I'm doing all sorts of not-recomended things to break out of Docker's attempts at confining plugins.


## Usage

Once running, you'll be able to use the `swarm` volume type just like you do local storage, but it will be accessible to any container, on any node.
multi-container access works, but there is no file-lock sharing, so you do need to deal with collisions in your code.

### Prerequisites

A Docker swarm.

### Installation

#### The simplest way

```
docker node update --label-add ${STACKDOMAIN:-loc.alho.st}-seaweedfs=true <one of your swarm master nodes>
docker stack deploy -c seaweedfs.yml seaweedfs
```

This will start 
* a persistent volume server on every swarm node, 
* a persistent filer service on a master node with `${STACKDOMAIN:-loc.alho.st}-seaweedfs=true` set as a label
* a non-persistent master node, when it moves, it will rebuild its data from the volume containers
* an s3 container that talks to the filer (untested by me)
* and a global run-once service that will install the volume plugin on every node, ready to be used by other swarm stacks.

To update the plugin on every node, you can run (you only need to do this once, it will go out to every node)

```
docker service update --force seaweedfs_docker-volume-plugin-run-once
```

#### manually, assuming you already have a seaweedfs swarm stack running

```
sven@t440s:~$ docker plugin ls
ID                  NAME                DESCRIPTION         ENABLED
sven@t440s:~$ docker plugin install --alias swarm svendowideit/seaweedfs-volume-plugin:next DEBUG=true
Plugin "svendowideit/seaweedfs-volume-plugin:next" is requesting the following privileges:
 - network: [host]
 - mount: [/var/lib/docker/plugins/]
 - mount: [/run/docker.sock]
 - device: [/dev/fuse]
 - capabilities: [CAP_SYS_ADMIN]
Do you grant the above permissions? [y/N] y
next: Pulling from svendowideit/seaweedfs-volume-plugin
51eeeee7b008: Download complete 
Digest: sha256:5a50736c3b6fa574e03638f4195f6175e8691818fdccf10e0d07e59813af494b
Status: Downloaded newer image for svendowideit/seaweedfs-volume-plugin:next
Installed plugin svendowideit/seaweedfs-volume-plugin:next
sven@t440s:~$ 
sven@t440s:~$ 
sven@t440s:~$ docker volume create -d swarm test
test
sven@t440s:~$ docker run --rm -it -v test:/test debian sh
# ls test
date.txt  etc
# 
```

## How it works.

The Plugin bindmounts in the host's Docker socket and `/var/lib/docker/plugins` dir. It uses this to work out what its called, and where it is supposed to mount files to. This allows the plugin to create intermediate containers that can access the seaweedfs_internal network to talk to the seaweedfs filer and volume services.

Right now, the `seaweedfs.yml` services compose file starts the seaweedfs services, and a "run-once" global service that installs this plugin on the other swarm nodes.

### Eventually

I want the plugin to start and update the seaweedfs swarm stack, and to listen to swarm events. This should allow it to dynamically change the
`defaultReplication` setting (presumably relative to the number of swarm masters), to send out the `re-balance` API signal, and keep things going.

I also want to see if I can make a Docker cli plugin that talks to the volume plugin to allow the user to use the docker cli to make seaweedfs config changes.

