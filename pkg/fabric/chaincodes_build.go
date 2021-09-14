package fabric

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/docker/buildx/build"
	"github.com/docker/buildx/util/progress"
	"github.com/docker/cli/cli/command"
	clitypes "github.com/docker/cli/cli/config/types"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/jsonmessage"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/timoth-y/fabnctl/pkg/docker"
	"github.com/timoth-y/fabnctl/pkg/kube"
	"github.com/timoth-y/fabnctl/pkg/ssh"
	"github.com/timoth-y/fabnctl/pkg/term"
	"k8s.io/kubectl/pkg/cmd/util"
)

func (c *Chaincode) Build(ctx context.Context, sourcePath string, options ...BuildOption) error {
	var (
		args = &buildArgs{
			sourcePath: sourcePath,
			sourcePathAbs: sourcePath,
			useSSH: true,
		}

		err error
	)

	for j := range options {
		options[j](args)
	}

	if len(args.initErrors) > 0 {
		return fmt.Errorf(util.MultipleErrors("invalid args", args.initErrors))
	}

	if args.sourcePathAbs, err = filepath.Abs(sourcePath); err != nil {
		return fmt.Errorf("absolute path '%s' of source does not exists: %w", args.sourcePathAbs, err)
	}

	if args.useSSH {

	}

	switch {
	case args.useSSH:
		return c.buildSSH(ctx, args)
	case args.useDocker:
		return c.buildDocker(ctx, args)
	}

	return nil
}

func (c *Chaincode) buildSSH(ctx context.Context, args *buildArgs) error {
	var (
		srcHash    = md5.Sum([]byte(args.sourcePathAbs))
		remotePath = filepath.Join("/tmp/fabnctl/build", hex.EncodeToString(srcHash[:]))
		buildCmd   string
	)

	if err := args.sshOperator.Transfer(args.sourcePathAbs, remotePath,
		ssh.WithContext(ctx),
		ssh.WithConcurrency(10),
		ssh.WithSkip(args.ignore...),
	); err != nil {
		return err
	}

	if _, err := os.Stat(filepath.Join(args.sourcePathAbs, "BUILD")); os.IsNotExist(err)  {
		buildCmd = kube.FormCommand("docker", "build",
			"-t", c.imageName,
			"-f", filepath.Join(remotePath, args.dockerfile),
			remotePath,
		)

		if args.pushImage {
			buildCmd = kube.FormCommand(buildCmd, "--push")
		}
	} else {
		buildCmd = kube.FormCommand(
			"cd", remotePath,
			"&&",
			"bazel", "run", fmt.Sprintf("//smartcontracts/%s:image", c.chaincodeName),
		)

		if args.pushImage {
			buildCmd = kube.FormCommand(buildCmd, "&&",
				"bazel", "run", fmt.Sprintf("//smartcontracts/%s:image-push", c.chaincodeName),
			)
		}
	}

	if _, _, err := args.sshOperator.Execute(buildCmd, ssh.WithStream(true)); err != nil {
		return err
	}

	return nil
}

func (c *Chaincode) buildDocker(ctx context.Context, args *buildArgs) error {
	var (
		platform   = fmt.Sprintf("linux/%s", c.arch)
		printer    = progress.NewPrinter(ctx, os.Stdout, "auto")
	)

	if _, err := build.Build(ctx, args.dockerDriver, map[string]build.Options{
		"default": {
			Platforms: []v1.Platform{{
				Architecture: c.arch,
				OS:           "linux",
			}},
			Tags: []string{c.imageName},
			Inputs: build.Inputs{
				ContextPath:    args.sourcePathAbs,
				DockerfilePath: path.Join(args.sourcePathAbs, args.dockerfile),
			},
		},
	}, docker.API(), docker.CLI.ConfigFile(), printer); err != nil {
		return fmt.Errorf("failed to build chaincode image from source path: %w", err)
	}

	_ = printer.Wait()

	c.logger.Successf("Successfully built chaincode image and tagged it: %s", c.imageName)

	// Pushing chaincode image to registry
	if !args.pushImage {
		return nil
	}

	if err := c.determineDockerCredentials(args); err != nil {
		return err
	}

	c.logger.Infof("Pushing chaincode image to '%s' registry", args.dockerRegistry)

	resp, err := docker.Client.ImagePush(ctx, c.imageName, types.ImagePushOptions{
		Platform:     platform,
		RegistryAuth: args.dockerRegistry,
		All:          true,
	})
	if err != nil {
		return fmt.Errorf("failed to push chaincode image to '%s' registry: %w", args.dockerRegistry, err)
	}

	_ = jsonmessage.DisplayJSONMessagesToStream(resp, docker.CLI.Out(), nil)

	c.logger.Successf("Chaincode image '%s' has been pushed to registry", c.imageName)

	return nil
}

func (c *Chaincode) determineDockerCredentials(args *buildArgs) error {
	var (
		err      error
		hostname = "https://index.docker.io/v1/"
	)

	dockerCredentials, _ := docker.CLI.ConfigFile().GetAllCredentials()
	if dockerCredentials == nil {
		dockerCredentials = map[string]clitypes.AuthConfig{}
	}

	if strings.Contains(args.dockerRegistry, ".") {
		hostname = fmt.Sprintf("https://%s/", args.dockerRegistry)
	}

	if len(args.dockerAuth) != 0 {
		auth := types.AuthConfig{ServerAddress: hostname}
		if identity := strings.Split(args.dockerAuth, ":"); len(identity) == 2 {
			auth.Username = identity[0]
			auth.Password = identity[1]
		} else {
			auth.IdentityToken = args.dockerAuth
		}

		if args.dockerAuth, err = command.EncodeAuthToBase64(auth); err != nil {
			return fmt.Errorf("failed to encode registry auth: %w", err)
		}

		return nil
	}

	identity, ok := dockerCredentials[hostname]
	if !ok {
		return fmt.Errorf("%w: credentials for '%s' not found in docker config and missing in args",
			term.ErrInvalidArgs, args.dockerRegistry)
	}

	if payload, err := json.Marshal(identity); err != nil {
		return fmt.Errorf("failed to encode registry auth: %w", err)
	} else {
		args.dockerAuth = base64.StdEncoding.EncodeToString(payload)
	}

	if len(args.dockerRegistry) == 0 {
		args.dockerRegistry = identity.Username
	}

	return nil
}
