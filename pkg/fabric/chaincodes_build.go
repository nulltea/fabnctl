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
	"github.com/pkg/errors"
	"github.com/timoth-y/fabnctl/cmd/fabnctl/shared"
	"github.com/timoth-y/fabnctl/pkg/docker"
	"github.com/timoth-y/fabnctl/pkg/kube"
	"github.com/timoth-y/fabnctl/pkg/ssh"
	"k8s.io/kubectl/pkg/cmd/util"
)

func (i *ChaincodeInstaller) Build(ctx context.Context, sourcePath string, options ...BuildOption) error {
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
		return i.buildSSH(ctx, args)
	case args.useDocker:
		return i.buildSSH(ctx, args)
	}

	return nil
}

func (i *ChaincodeInstaller) buildSSH(ctx context.Context, args *buildArgs) error {
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
			"-t", i.imageName,
			"-f", filepath.Join(remotePath, args.dockerfile),
			"--push",
			remotePath,
		)
	} else {
		buildCmd = kube.FormCommand(
			"cd", remotePath,
			"&&",
			"bazel", "run", fmt.Sprintf("//smartcontracts/%s:image", i.chaincodeName),
		)
	}

	if _, stderr, err := args.sshOperator.Execute(buildCmd, ssh.WithStream(true)); err != nil {
		return err
	} else if len(stderr) != 0 {
		return fmt.Errorf("failed building chaincode: %s", string(stderr))
	}

	return nil
}

func (i *ChaincodeInstaller) buildDocker(ctx context.Context, args *buildArgs) error {
	var (
		platform   = fmt.Sprintf("linux/%s", i.arch)
		printer    = progress.NewPrinter(ctx, os.Stdout, "auto")
	)

	if _, err := build.Build(ctx, args.dockerDriver, map[string]build.Options{
		"default": {
			Platforms: []v1.Platform{{
				Architecture: i.arch,
				OS:           "linux",
			}},
			Tags: []string{i.imageName},
			Inputs: build.Inputs{
				ContextPath:    args.sourcePathAbs,
				DockerfilePath: path.Join(args.sourcePathAbs, args.dockerfile),
			},
		},
	}, docker.API(), docker.CLI.ConfigFile(), printer); err != nil {
		return fmt.Errorf("failed to build chaincode image from source path: %w", err)
	}

	_ = printer.Wait()

	// cmd.Printf("\n%s Successfully built chaincode image and tagged it '%s'\n",
	// 	viper.GetString("cli.success_emoji"), imageTag,
	// )

	// Pushing chaincode image to registry
	if !args.dockerPush {
		return nil
	}

	if err := i.determineDockerCredentials(args); err != nil {
		return err
	}

	// cmd.Printf("\n🚀 Pushing chaincode image to '%s' registry\n\n", registry)

	resp, err := docker.Client.ImagePush(ctx, i.imageName, types.ImagePushOptions{
		Platform:     platform,
		RegistryAuth: args.dockerRegistry,
		All:          true,
	})
	if err != nil {
		return fmt.Errorf("failed to push chaincode image to '%s' registry: %w", args.dockerRegistry, err)
	}

	_ = jsonmessage.DisplayJSONMessagesToStream(resp, docker.CLI.Out(), nil)

	// cmd.Printf("\n%s Chaincode image '%s' has been pushed to registry\n",
	// 	viper.GetString("cli.success_emoji"), i.imageName,
	// )

	return nil
}

func (i *ChaincodeInstaller) determineDockerCredentials(args *buildArgs) error {
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
		return errors.Wrapf(
			shared.ErrInvalidArgs,
			"credentials for '%s' not found in docker config and missing in args", args.dockerRegistry,
		)
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
