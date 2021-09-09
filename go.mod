module github.com/timoth-y/fabnctl

go 1.16

require (
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/docker/buildx v0.5.1
	github.com/docker/cli v20.10.5+incompatible
	github.com/docker/docker v20.10.5+incompatible
	github.com/docker/libnetwork v0.8.0-dev.2.0.20201215162534-fa125a3512ee // indirect
	github.com/gernest/wow v0.1.0
	github.com/kr/fs v0.1.0 // indirect
	github.com/manifoldco/promptui v0.8.0
	github.com/mittwald/go-helm-client v0.5.0
	github.com/moby/buildkit v0.8.3 // indirect
	github.com/moby/sys/mount v0.2.0 // indirect
	github.com/moby/term v0.0.0-20201110203204-bea5bbe245bf // indirect
	github.com/op/go-logging v0.0.0-20160315200505-970db520ece7
	github.com/opencontainers/image-spec v1.0.1
	github.com/pkg/errors v0.9.1
	github.com/pkg/sftp v1.13.3
	github.com/spf13/cobra v1.1.3
	github.com/spf13/viper v1.7.0
	golang.org/x/crypto v0.0.0-20210513164829-c07d793c2f9a
	golang.org/x/sys v0.0.0-20210906170528-6f6e22806c34 // indirect
	k8s.io/api v0.20.6
	k8s.io/apimachinery v0.20.6
	k8s.io/client-go v0.20.6
	k8s.io/kubectl v0.20.1
	sigs.k8s.io/yaml v1.2.0
)

replace github.com/jaguilar/vt100 => github.com/tonistiigi/vt100 v0.0.0-20190402012908-ad4c4a574305
