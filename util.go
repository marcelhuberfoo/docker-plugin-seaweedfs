package main

import (
	"context"
	"io/ioutil"

	"github.com/docker/cli/cli/connhelper"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
	// TODO: beware, this is archived
)

// Worth reading: https://docs.docker.com/engine/api/v1.24/
// and https://docs.docker.com/engine/api/v1.27/#operation/ContainerCreate

func GetDockerClient(ctx context.Context, host string) (*client.Client, error) {
	// TODO: the docker-ce ssh helper requires code in the docker daemon 18.09
	//       change this to use pure ssh tunneled unix sockets so it can be any version
	var err error
	var cli *client.Client
	if host != "" {
		var helper *connhelper.ConnectionHelper

		helper, err = connhelper.GetConnectionHelper(host)
		if err != nil {
			return nil, err
		}
		cli, err = client.NewClientWithOpts(
			client.WithHost(helper.Host),
			client.WithDialContext(helper.Dialer),
		)
	} else {
		cli, err = client.NewClientWithOpts(
			client.FromEnv,
		)

	}
	if err != nil {
		return nil, err
	}
	cli.NegotiateAPIVersion(ctx)

	return cli, err
}

func runContainer(
	config *container.Config,
	hostConfig *container.HostConfig,
	networkingConfig *network.NetworkingConfig,
	containerName string,
) (string, error) {
	ctx := context.Background()
	cli, err := GetDockerClient(ctx, "")
	if err != nil {
		logError("Error getting Docker client: %s", err)
		return "", err
	}
	reader, err := cli.ImagePull(ctx, config.Image, types.ImagePullOptions{})
	if err != nil {
		logError("Error pulling Container: %s", err)
		return "", err
	}
	//io.Copy(os.Stdout, reader)
	b, err := ioutil.ReadAll(reader)
	logrus.Debugf("ImagePull(%s): (Err: %s ) Output: %s", config.Image, err, b)
	if err != nil {
		logError("ImagePull: %s", err)
		return "", err
	}

	cResponse, err := cli.ContainerCreate(ctx,
		config,
		hostConfig,
		networkingConfig,
		containerName,
	)
	if err != nil {
		logError("Error creating Container: %s", err)
		return "", err
	}
	if err = cli.ContainerStart(ctx, cResponse.ID, types.ContainerStartOptions{}); err != nil {
		logError("Error starting Container: %s", err)
		return "", err
	}

	return cResponse.ID, nil
}
