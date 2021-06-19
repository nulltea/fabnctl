package shared

import (
	docker "github.com/docker/docker/client"
)

var Docker *docker.Client

func initDocker() {
	var err error

	Docker, err = docker.NewClientWithOpts(docker.FromEnv)
	if err != nil {
		panic(err)
	}
}
