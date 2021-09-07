package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/docker/buildx/build"
	"github.com/docker/buildx/driver"
	_ "github.com/docker/buildx/driver/docker"
	_ "github.com/docker/buildx/driver/docker-container"
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	docker "github.com/docker/docker/client"
	"github.com/pkg/errors"
)

var (
	Docker *docker.Client
	DockerCLI *command.DockerCli
)

func initDocker() {
	var err error

	Docker, err = docker.NewClientWithOpts(docker.FromEnv)
	if err != nil {
		panic(err)
	}

	if DockerCLI, err = command.NewDockerCli(); err != nil {
		panic(err)
	}

	if err := DockerCLI.Initialize(flags.NewClientOptions(), command.WithInitializeClient(func(dockerCli *command.DockerCli) (docker.APIClient, error) {
		return Docker, err
	})); err != nil {
		panic(err)
	}
}

func DockerAPI() *api {
	return &api{}
}

type api struct {}

func (a *api) DockerAPI(name string) (docker.APIClient, error) {
	return docker.APIClient(Docker), nil
}

func DockerBuildDrivers(ctxPath string) ([]build.DriverInfo, error) {
	d, err := driver.GetDriver(
		context.Background(),
		"buildx_buildkit_default",
		nil, DockerCLI.Client(), DockerCLI.ConfigFile(),
		nil, nil, "", nil, nil, ctxPath,
	)

	if err != nil {
		return nil, err
	}

	return []build.DriverInfo{
		{
			Name:   "default",
			Driver: d,
		},
	}, nil
}

func ParseDockerResponse(reader io.ReadCloser) (io.Reader, error) {
	var (
		err error
		output bytes.Buffer
		d = json.NewDecoder(reader)
	)


	type Event struct {
		Stream      *string `json:"stream"`
		ErrorDetail *struct{
			Message string `json:"message"`
		} `json:"errorDetail,omitempty"`
	}

	var event *Event
	for {
		if err := d.Decode(&event); err != nil {
			if err == io.EOF {
				break
			}

			panic(err)
		}

		if event.Stream != nil {
			fmt.Fprint(&output, *event.Stream)
		}

		if event.ErrorDetail != nil {
			err = errors.New(event.ErrorDetail.Message)
			break
		}
	}

	reader.Close()

	return &output, err
}
