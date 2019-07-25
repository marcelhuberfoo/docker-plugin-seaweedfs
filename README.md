# SeaweedFS Docker Plugin

This is stupendously experimental. I'm doing all sorts of not-recomended things to break out of Docker's attempts at confining plugins.



## Usage

Once running, you'll be able to use the `swarm` volume type just like you do local storage, but it will be accessible to any container, on any node.
multi-container access works, but there is no file-lock sharing, so you do need to deal with collisions in your code.

### Prerequisites

A Docker swarm.

### Installation

#### The simplest way

`docker stack deploy -c seaweedfs.yml seaweedfs`

This will start 
* a persistent volume server on every swarm node, 
* a persistent filer service on one node (and when it moves, you'll lose access to your data :( )
* a non-persistent master node, when it moves, it will rebuild its data from the volume containers
* an s3 container that talks to the filer (untested by me)
* and a global run-once service that will install the volume plugin on every node, ready to be used by other swarm stacks.

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

### Features

#### Shared Mounts

Any number of containers on any number of hosts can mount the same volume at the same time. The only requirement is that each Docker host have the SeaweedFS plugin installed on it.

#### Transparent Data Storage ( No Hidden Metadata )

Each SeaweedFS Docker volume maps 1-to-1 to a directory on the SeaweedFS filesystem. All directories in the [REMOTE_PATH](#remote-path) on the SeaweedFS filesystem will be exposed as a Docker volume regardless of whether or not the directory was created by running `docker volume create`. There is no special metadata or any other extra information used by the plugin to keep track of what volumes exist. If there is a directory there, it is a Docker volume and it can be mounted ( and removed ) by the SeaweedFS plugin. This makes it easy to understand and allows you to manage your Docker volumes directly on the filesystem, if necessary, for things like backup and restore.

#### Multiple SeaweedFS Clusters

It is also possible, if you have multiple SeaweedFS clusters, to install the plugin multiple times with different settings for the different clusters. For example, if you have two SeaweedFS clusters, one at `host1` and another at `host2`, you can install the plugin two times, with different aliases, to allow you to create volumes on both clusters.

    $ docker plugin install --alias seaweedfs1 --grant-all-permissions katharostech/seaweedfs-volume-plugin HOST=host1:8888
    $ docker plugin install --alias seaweedfs2 --grant-all-permissions kadimasolutions/seaweedfs-volume-plugin HOST=host2:8888

This gives you the ability to create volumes for both clusters by specifying either `seaweedfs1` or `seaweedfs2` as the volume driver when creating a volume.

#### Root Mount Option

The plugin has the ability to provide a volume that contains *all* of the SeaweedFS Docker volumes in it. This is called the Root Volume and is identical to mounting the configured `REMOTE_PATH` on your SeaweedFS filesystem into your container. This volume does not exist by default. The Root Volume is enabled by setting the `ROOT_VOLUME_NAME` to the name that you want the volume to have. You should pick a name that does not conflict with any other volume. If there is a volume with the same name as the Root Volume, the Root Volume will take precedence over the other volume.

There are a few different uses for the Root Volume. Katharos Technology designed the Root Volume feature to accommodate for containerized backup solutions. By mounting the Root Volume into a container that manages your Backups, you can backup *all* of your SeaweedFS Docker volumes without having to manually add a mount to the container every time you create a new volume that needs to be backed up.

The Root Volume also give you the ability to have containers create and remove SeaweedFS volumes without having to mount the Docker socket and make Docker API calls. Volumes can be added, removed, and otherwise manipulated simply by mounting the Root Volume and making the desired changes.

## Configuration

### Plugin Configuration

You can configure the plugin through plugin variables. You may set these variables at installation time by putting `VARIABLE_NAME=value` after the plugin name, or you can set them after the plugin has been installed using `docker plugin set katharostech/seaweedfs-volume-plugin VARIABLE_NAME=value`.

> **Note:** When configuring the plugin after installation, the plugin must first be disabled before you can set variables. There is no danger of accidentally setting variables while the plugin is enabled, though. Docker will simply tell you that it is not possible.

#### HOST

The hostname/ip address and port that will be used when connecting to the SeaweedFS filer.

> **Note:** The plugin runs in `host` networking mode. This means that even though it is in a container, it shares its network configuration with the host and should resolve all network addresses as the host system would.

**Default:** `localhost:8080`

#### MOUNT_OPTIONS

Options passed to the `weed mount` command when mounting SeaweedFS volumes.

**Default:** empty string

#### REMOTE_PATH

The path on the SeaweedFS filesystem that Docker volumes will be stored in. This path will be mounted for volume storage by the plugin and must exist on the SeaweedFS filesystem.

**Default:** `/docker/volumes`

#### ROOT_VOLUME_NAME

The name of the Root Volume. If specified, a special volume will be created of the given name will be created that will contain all of the SeaweedFS volumes. It is equivalent to mounting the whole of `REMOTE_PATH` on the SeaweedFS filesystem. See [Root Mount Option](#root-mount-option).

**Default:** empty string

#### LOG_LEVEL

Plugin logging level. Set to `DEBUG` to get more verbose log messages. Logs from Docker plugins can be found in the Docker log and will be suffixed with the plugin ID.

**Default:** `INFO`

## Development

Docker plugins are made up of a `config.json` file and `rootfs` directory. The `config.json` has all of the metadata and information about the plugin that Docker needs when installing and configuring the plugin. The `rootfs` is the root filesystem of the plugin container. Unfortunately the Docker CLI doesn't allow you to create Docker plugins using a Dockerfile so we use a Makefile to automate the process of creating the plugin `rootfs` from a Dockerfile.

### Building the Plugin

To build the plugin simply run `make rootfs` in the project directory.

    $ make rootfs

This will build the Dockerfile, export the new Docker image's rootfs, and copy the rootfs and the config.json file to the `plugin` directory. When it is done you should have a new plugin directory with a config.json file and a rootfs folder in it.

```
plugin/
  config.json
  rootfs/
```

After that is finished you can run `make create`.

    $ make create

This will install the Docker plugin from the `plugin` dirctory with the name `katharostech/seaweedfs-volume-plugin`.

Finally run `make enable` to start the plugin.

    $ make enable

 Here is a list of the `make` targets:

* **clean**: Remove the `plugin` directory
* **config**: Copy the `config.json` file to the `plugin` directory
* **rootfs**: Generate the plugin rootfs from the Dockerfile and put it in the `plugin` directory with the `config.json`
* **create**: Install the plugin from the `plugin` directory
* **enable**: Enable the plugin
* **disable**: Disable the plugin
* **push**: Run the `clean`, `rootfs`, `create`, and `enable` targets, and push the plugin to DockerHub

### Running the tests

> **Note:** The tests have not be migrated from the LizardFS version of this plugin. The information in this section about tests is straight from the LizardFS version and hasn't been tested after porting the plugin.

The automated tests for the plugin are run using a Docker-in-Docker container that creates a Dockerized SeaweedFS cluster to test the plugin against. When you run the test container, it will install the plugin inside the Docker-in-Docker container and proceed to create a Dockerized LizardFS cluster in it as well. A shell script is run that manipulates the plugin and runs containers to ensure the plugin behaves as is expected.

Before you can run the tests, the test Docker image must first be built. This is done by running the `build-tests.sh` script.

    $ ./build-tests.sh

This will build a Docker image, `lizardfs-volume-plugin_test`, using the Dockerfile in the `test` directory. After the image has been built, you can use it to run the tests against the plugin. This is done with the `run-tests.sh` script.

    $ ./run-tests.sh

By default running `run-tests.sh` will install the plugin from the `plugin` directory before running the tests against it. This means that you must first build the plugin by running `make rootfs`, if you have not already done so. Alternatively, you can also run the tests against a version of the plugin from DockerHub by passing in the plugin tag as a parameter to the `run-tests.sh` script.

    $ ./run-tests.sh kadimasolutions/lizardfs-volume-plugin:latest

This will download the plugin from DockerHub and run the tests against that version of the plugin.

### Tips & Tricks

If you don't have a fast disk on your development machine, developing Docker plugins can be somewhat tricky, because it can take some time to build and install the plugin every time you need to make a change. Here are some tricks that you can use to help maximize your development time.

#### Patching the Plugin Rootfs

All of the plugin logic is in the `index.js` file. During development it can take a long time to rebuild the entire plugin every time you need to test a change to `index.js`. To get around this, it is possible to copy just that file into the installed plugin without having to reinstall the entire plugin.

When you install a Docker plugin, it is given a plugin ID. You can see the first 12 characters of the plugin ID by running `docker plugin ls`.

```
$ docker plugin ls
ID                  NAME                                            DESCRIPTION                         ENABLED
2f5b68535b92        katharostech/seaweedfs-volume-plugin:latest   SeaweedFS volume plugin for Docker   false
```

Using that ID you can find where the plugin's rootfs was installed. By default, it should be located in `/var/lib/docker/plugins/[pluginID]/rootfs`. For our particular plugin, the file that we need to replace is the `/project/index.js` file in the plugin's rootfs. By replacing that file with an updated version and restarting ( disabling and re-enabling ) the plugin, you can update the plugin without having to re-install it.

#### Exec-ing Into the Plugin Container

It may be useful during development to exec into the plugin container while it is running. You can find out how in the [Docker Documentation](https://docs.docker.com/engine/extend/#debugging-plugins).

#### Test Case Development

> **Note:** The tests have not be migrated from the LizardFS version of this plugin. The information in this section about tests is straight from the LizardFS version and hasn't been tested after porting the plugin.

Writing new automated test cases for the plugin can also be difficult because of the time required for the test container to start. When writing new test cases for the plugin, it may be useful to start the container and interactively run the tests. If you make a mistake that causes a test to fail, even though the plugin *is* working, you can still edit and re-run the tests without having to restart the test container completely.

Once you have built the test image using the `build-tests.sh` script, you need to run the test container as a daemon that you can exec into. We override the entrypoint of the container so that it won't run the test script as soon as it starts. We want it just to sit there and wait for us to run commands in it.

    $ docker run -it --rm -d --name lizardfs-test --privileged \
    -v $(pwd)/plugin:/plugin \
    -v $(pwd)/test/test-run.sh:/test-run.sh \
    --entrypoint=sh \
    lizardfs-volume-plugin_test

> **Note:** We also mount our `test-run.sh` script into the container so that updates to the script are reflected immediately in the container.

After the container is running we can shell into it and run the script that starts up Docker.

    $ docker exec -it lizardfs-test sh
    /project # /test-environment.sh

This will start Docker, load the LizardFS image used for creating the test LizardFS environment, and install the plugin from the plugin directory. Once this is done you can run the tests.

    /project # sh /test-run.sh

This will run through all of the tests. If the tests fail, you can still edit and re-run the `test-run.sh` script without having to re-install the plugin.

When you are done writing your test cases, you can `exit` the shell and `docker stop lizardfs-test`. The container will be automatically removed after it stops. You should make sure that your tests still run correctly in a completely fresh environment by rebuilding and re-running the tests using the `build-tests.sh` and `run-tests.sh` scripts.
