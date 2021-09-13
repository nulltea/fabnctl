package docker

import (
	"context"

	"github.com/docker/buildx/build"
	"github.com/docker/buildx/driver"
)

func BuildDrivers(ctxPath string) ([]build.DriverInfo, error) {
	d, err := driver.GetDriver(
		context.Background(),
		"buildx_buildkit_default",
		nil, CLI.Client(), CLI.ConfigFile(),
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
