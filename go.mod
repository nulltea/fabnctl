module github.com/timoth-y/fabnctl

go 1.16

require (
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/containerd/console v1.0.1
	github.com/containerd/containerd v1.5.0-beta.4 // indirect
	github.com/docker/buildx v0.5.1
	github.com/docker/cli v20.10.5+incompatible
	github.com/docker/docker v20.10.5+incompatible
	github.com/docker/libnetwork v0.8.0-dev.2.0.20201215162534-fa125a3512ee // indirect
	github.com/gernest/wow v0.1.0
	github.com/manifoldco/promptui v0.8.0
	github.com/mittwald/go-helm-client v0.5.0
	github.com/moby/buildkit v0.8.3 // indirect
	github.com/moby/sys/mount v0.2.0 // indirect
	github.com/moby/term v0.0.0-20201110203204-bea5bbe245bf // indirect
	github.com/op/go-logging v0.0.0-20160315200505-970db520ece7
	github.com/opencontainers/image-spec v1.0.1
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.1.3
	github.com/spf13/viper v1.7.0
	github.com/stretchr/testify v1.7.0 // indirect
	github.com/timoth-y/chainmetric-core v0.0.0-20210531011854-b6d190056d3e
	golang.org/x/crypto v0.0.0-20210513164829-c07d793c2f9a // indirect
	golang.org/x/term v0.0.0-20210615171337-6886f2dfbf5b // indirect
	google.golang.org/grpc v1.35.0 // indirect
	k8s.io/api v0.20.6
	k8s.io/apimachinery v0.20.6
	k8s.io/client-go v0.20.6
	k8s.io/kubectl v0.20.1
	sigs.k8s.io/yaml v1.2.0
)

replace github.com/jaguilar/vt100 => github.com/tonistiigi/vt100 v0.0.0-20190402012908-ad4c4a574305
