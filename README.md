# fabnctl

_`fabnctl` is a developer friendly CLI for Hyperledger Fabric on your Kubernetes infrastructure._

![cmd help image]

[cmd help image]: https://github.com/timoth-y/fabnctl/blob/github/update_readme/docs/fabnctl.png?raw=true

## Intentions

[Hyperledger Fabric][hyperledger fabric url] is a powerful enterprise-grade permissioned distributed ledger framework. 
Its modular architecture and unique orderer-based approach to consensus enable versatility in use cases and production-ready performance and scalability.

However, its deployment procedure especially in the Kubernetes environment may require quite a lot of time and effort.
Hyperledger Fabric's documentation is indeed helpful though it still covers the very basics.

`fabnctl` command line utility provides a straightforward and convenient way of deploying permissioned blockchain network in the Kubernetes environment.

Its developer-first approach attempts to eliminate initial struggle when starting on a project with Hyperledger Fabric onboard,
as well at further scaling and chaincode (Smart Contracts) development process.

In addition, its emoji-rich and interactive smooth logging will keep you calm and joyful throughout the entire ride into
the world of deterministic blockchain technology, contract-oriented development, and brave Byzantine battles.

[hyperledger fabric url]: https://www.hyperledger.org/use/fabric

## Foreword

This repository was derived from the [`timoth-y/chainmetric-network`][chainmetric-network],
the development of which required a more convenient way of its blockchain infrastructure deployment.
Consider checking it out too.

[chainmetric-network]: https://github.com/timoth-y/chainmetric-network

## Requirements

- Existing Kubernetes environment, ARM-based clusters, K3s, and [minikube][minikube] is also suitable
- Configured `kubectl` with connection to your, `.kube` config is expected to be located in $HOME directory
- Volume provisioner installed in K8s cluster, this projects intends to use [`rancher/local-path-provisioner`](https://github.com/rancher/local-path-provisioner),
  you can use `make prepare-cluster` for this. Alternatively it is possible to modify Storage Class in `.cli-config.yaml`
- Reverse proxy installed in K8s cluster, this projects intends to use [Traefik](https://github.com/traefik/traefik).
  You can install it with [Helm chart](https://github.com/traefik/traefik-helm-chart) or use `make prepare-cluster` rule.
- Docker engine installed on your local device, it's required for building chaincode images

## Installation

Download the latest release:

```shell
curl --location "https://github.com/timoth-y/fabnctl/releases/latest/download/fabnctl_$(uname -s)_amd64.tar.gz" | tar xz -C /tmp
```

For ARM system, please change ARCH (e.g. armv6, armv7 or arm64) accordingly:

```shell
curl --location "https://github.com/timoth-y/fabnctl/releases/latest/download/fabnctl_$(uname -s)_arm64.tar.gz" | tar xz -C /tmp
```

Install tool:

```shell
cd /tmp/fabnctl && make install
```

## Usage

### Cluster preparation

Before starting deploying Fabric on your cluster, check if all requirements met. 
You can use `prepare-cluster` rule to install all required charts on your cluster:

```shell
make prepare-cluster
```

### Network configuration

Now when we sure cluster is ready to go, first thing to do is fill the network configuration based on your needs:

```shell
nano network-config.yaml # See network-config-example.yaml for example
```

### Generate artifacts

Okay, one more thing before deploying an actual Fabric components is to generate crypto-materials and channel artifacts:

```shell
fabnctl gen artifacts --arch=arm64 --domain=example.network -f ./network-config.yaml
```

![gen artifacts gif]

This will generate all required artifacts based on configuration in `network-config.yaml` in shared persistent volume,
as well as download it in your local device to the following directories: `.channel-artifacts.$DOMAIN` and `.crypto-config.$DOMAIN`.

### Deploy orderer

The essential components of Hyperledger Fabric blockchain is of course [Ordering Service][orderer],
which is responsible for ensuring data consistency and enables performance at scale while preserving privacy.

Though, its deployment is actually a piece of cake:

```shell
fabnctl deploy orderer --arch=arm64 --domain=example.network
```

![deploy orderer gif]

> Hyperledger foundation does not provide official images for ARM-based systems.
> Instead this project currently use alternative images found on DockerHub.
> It's planned to build and source own images latter on to keep up with Fabric releases.
> Unless of course Hyperledger decides to include support for ARM, which may indeed be highly possibly since the rise of
> single-board ARM based computers such as Raspberry Pi and others.

### Deploy organization peers

Great, moving on to [organizations][organization] and [peers][peer] owned by them,
which will store a copy of ledger and perform read/write operations on it.

To deploy peer for certain organization use following command:

```shell
fabnctl deploy peer --arch=arm64 --domain=example.network --org=org1 --peer=peer0
```

![deploy peer gif]

Along with peer this command will deploy `cli.$peer.$org.$domain` pod, `ca.$org.$domain` (can be skipped with `--withCA=false`),
and `couchdb.$peer.$org.$domain` in case that state database is specified in the `network-config.yaml`.

### Deploy and join channels

Now before adding functionality to the network, which is of course Smart Contracts,
one more crucially important thing is needed to be done - provide communication between peers aka [channels][channel].

Doing that also won't take much of your time and effort:

```shell
fabnctl deploy channel --domain=example.network --channel=example-channel \
   -o=org1 -p=peer0 \
   -o=org2 -p=peer0 \
   -o=org3 -p=peer0
```

![deploy channel gif]

That would create channel with specified name and join all given organization peers to it.

### Deploy chaincodes

Now we're talking! So, assuming your Smart Contract is written and ready to be tested in the distributed wilderness,
the following command will perform a sequence of actions to make that happened:

```shell
fabnctl deploy cc --arch=arm64 --domain=example.network --chaincode=example -C=example-channel \
   -o=org1 -p=peer0 \
   -o=org2 -p=peer0 \
   -o=org3 -p=peer0 \
   --registry=dockerhubuser ./chaincodes/example
```

![deploy cc gif]

Hope the gif loading haven't taken forever because this command takes some time. Under the hood it starts with building
docker image of the passed chaincode source code, which will then be pushed to the specified registry.
This step is required for deploying [chaincode as an external service][external cc], the feature that was introduced int Fabric v2.0,
and appear to be that best suitable for Kubernetes-based infrastructures.

Then the determination of the chaincode version and sequence takes place. For the initial deployment it will be v1.0, sequence 1.
During every next update that numbers would be incremented, but it is also possible to specify version with according flag.

The chaincode is then packed into the [`package.tar.gz`][cc package] and sent to the cli pods for installation and further approval.
This step is performed for each passed organization.

> It is important for chaincode to be approved by all organizations which are part of the channel.
> Thus, it is recommended to execute `deploy cc` command with all orgs passed. Otherwise, the commitment phase will fail.
> However, it is possible to split the process in batches, in thus scenario chaincode will be committed when last organization will approve it

### Set anchor peers on channel definition

One more thing to not forget about when deploying HLF network is to update channel to set anchor peers,
without this step organization peers won't be aware where the other organizations are hosted.

To prevent this causing troubles letter use this simple command now:

```shell
fabnctl update channel --setAnchors --domain=example.network  --channel=example-channel \
   -o=org1 \
   -o=org2 \
   -o=org3
```

![update channel gif]

### Bonus: Generate `connection.yaml`

Now, when the network is ready and functional the next logical step would be test it with some application,
which would require connection config.

This command line utility will gladly help you with that too:

```shell
fabnctl gen connection -f ./network-config.yaml --name application \
   --channel=example-channel --org=chipa-inu ./artifacts
```

![gen connection gif]

[gen artifacts gif]: https://github.com/timoth-y/fabnctl/blob/github/update_readme/docs/gen_artifacts.gif?raw=true
[deploy orderer gif]: https://github.com/timoth-y/fabnctl/blob/github/update_readme/docs/deploy_orderer.gif?raw=true
[deploy peer gif]: https://github.com/timoth-y/fabnctl/blob/github/update_readme/docs/deploy_peer.gif?raw=true
[deploy channel gif]: https://github.com/timoth-y/fabnctl/blob/github/update_readme/docs/deploy_channel.gif?raw=true
[deploy cc gif]: https://github.com/timoth-y/fabnctl/blob/github/update_readme/docs/deploy_cc.gif?raw=true
[update channel gif]: https://github.com/timoth-y/fabnctl/blob/github/update_readme/docs/update_channel.gif?raw=true
[gen connection gif]: https://github.com/timoth-y/fabnctl/blob/github/update_readme/docs/gen_connection.gif?raw=true

[crypto material]: https://hyperledger-fabric.readthedocs.io/en/release-2.2/identity/identity.html#digital-certificates
[channel]: https://hyperledger-fabric.readthedocs.io/en/release-2.2/glossary.html#channel
[orderer]: https://hyperledger-fabric.readthedocs.io/en/release-2.2/glossary.html#ordering-service
[peer]: https://hyperledger-fabric.readthedocs.io/en/release-2.2/glossary.html#peer
[organization]: https://hyperledger-fabric.readthedocs.io/en/release-2.2/glossary.html#organization
[chaincode]: https://hyperledger-fabric.readthedocs.io/en/release-2.2/glossary.html#smart-contract
[external cc]: https://hyperledger-fabric.readthedocs.io/en/release-2.2/cc_service.html
[cc package]: https://hyperledger-fabric.readthedocs.io/en/release-2.2/cc_service.html#packaging-chaincode

## Roadmap

- [ ] Add multiply orderers deployment support
- [ ] Simplify interactive logging implementation
- [ ] Source charts on [artifacthub.io](https://artifacthub.io)

## Contributing

Contributions are always welcome! Fork this repo to get started.

## Wrap up

The development of `fabnctl` utility was motivated by the need of providing more convenient and straightforward way of deploying
Hyperledger Fabric network for those who are getting started with this awesome technology
and potentially satisfy some needs of those who are already into Fabric's DevOps processes.

## License

Licensed under the [Apache 2.0][license file].

[license url]: https://www.apache.org/licenses/LICENSE-2.0

[minikube]:  https://minikube.sigs.k8s.io/docs/

[license file]: https://github.com/timoth-y/chainmetric-network/blob/main/LICENSE
