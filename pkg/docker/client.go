package docker

import (
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	"github.com/docker/docker/client"
)

var (
	Client *client.Client
	CLI    *command.DockerCli
)

func Init() {
	var err error

	Client, err = client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}

	if CLI, err = command.NewDockerCli(); err != nil {
		panic(err)
	}

	if err = CLI.Initialize(
		flags.NewClientOptions(),
		command.WithInitializeClient(func(dockerCli *command.DockerCli) (client.APIClient, error) {
			return Client, err
		},
	)); err != nil {
		panic(err)
	}
}

func API() *api {
	return &api{}
}

type api struct{}

func (a *api) DockerAPI(name string) (client.APIClient, error) {
	return client.APIClient(Client), nil
}


