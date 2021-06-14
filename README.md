# ChainMetric: Network

[![bash badge]][bash url]&nbsp;
[![blockchain badge]][hyperledger fabric url]&nbsp;
[![kubernetes badge]][kubernetes url]&nbsp;
[![license badge]][license url]

## Overview
_**Chainmetric Network**_ is an IoT-enabled permissioned blockchain network based on Hyperledger Fabric stack. 

It is oriented on storing, managing, and handling the continuous flow of sensor readings data, which is sourced by IoT on-network devices, and validating those readings against organization assigned requirements, thus providing full control on the assets supply chain.

This solution provides a convenient way of deploying such a blockchain network onto the cloud-based Kubernetes cluster environment via **Helm charts** and dedicated **Bash script**.

## Requirements

- Kubernetes cluster is required to deploy a Chainmetric network ([minikube][minikube] is also suitable).
- [Helm][helm] binaries must be presented on a local machine from which deployment script will be used.

## Deployment

[Hyperledger Fabric][hyperledger fabric url] is a powerful enterprise-grade permissioned distributed ledger framework. Its modular architecture and unique orderer-based approach to consensus enable versatility in use cases and production-ready performance and scalability.

However, its deployment procedure especially in the Kubernetes environment may require quite a lot of time and effort. Hyperledger Fabric's documentation is indeed helpful though it still covers the very basics. 

The current solution provides a straightforward way of deploying permissioned blockchain network in the Kubernetes environment using a combination of a Bash script and Helm charts with the following commands:

Use `init` command to generate [crypto materials][crypto material] and [network channels][network channel] artifacts:

```
$ ./network.sh init
```

Use `deploy` command with `orderer` action to deploy [Ordering Service][orderer service], which is responsible for ensuring data consistency and enables performance at scale while preserving privacy:
```
$ ./network.sh deploy orderer
```

Use `deploy` command with `peer` action to deploy single [Blockchain peer][blockchain peer] which will store a copy of ledger and perform read/write operations on it:
```
$ ./network.sh deploy peer --peer='peer subdomain name' --org='organization name'
```

Use `deploy` command with `channel` action to deploy [network channel][network channel] which provides a secure way of communication between peers:
```
$ ./network.sh deploy channel --channel='channel name' --peer='peer subdomain name' --org='organization name'
```

Use `deploy` command with `cc` action to deploy or upgrade [Chaincode][chaincode] (Smart Contract):
```
$ ./network.sh deploy cc --cc_name=`chaincode name` --channel='channel name' --peer='peer subdomain name' --org='organization name' --upgrade
```

Use following environmental variables to define some additional network properties:
```
$ export DOMAIN 'chainmetric.io'
$ export ORDERER 'orderer.chainmetric.io'
$ export CHAINCODES_DIR '../contracts'
$ export IMAGE_REGISTRY 'chainmetric'
```

## Roadmap

- [X] [CouchDB][couchdb] as the [World State][world state] database [(#2)](https://github.com/timoth-y/chainmetric-network/pull/2)
- [x] ~~[Kafka][kafka]~~ [Raft][raft] for [Ordering Service][orderer service]
- [x] Raspberry Pi (ARM64) deployement strategy [(#4)](https://github.com/timoth-y/chainmetric-network/pull/4)
- [ ] [Go][golang] written command utility or Kubernetes operator
- [ ] CI/CD integration (probably [GitLab CE][gitlab ci] or simply [GitHub Actions][github actions])
- [ ] Deploy [Hyperledger Explorer][hyperledger explorer] for managing and monitoring network from the web

## Wrap up

Chainmetric network designed to be an enterprise-grade, confidential and scalable distributed ledger, which in combination with dedicated [Smart Contracts][chainmetric contracts repo], embedded [sensor-equipped IoT][chainmetric sensorsys repo] devices, and cross-platform [mobile application][chainmetric app repo] provides ambitious metric requirements control solutions for general assets supply chains.

## License

Licensed under the [Apache 2.0][license file].



[bash badge]: https://img.shields.io/badge/Code-Bash-informational?style=flat&logo=gnu%20bash&logoColor=white&color=9DDE66
[blockchain badge]: https://img.shields.io/badge/Blockchain-Hyperledger%20Fabric-informational?style=flat&logo=hyperledger&logoColor=white&labelColor=0A1F1F&color=teal
[kubernetes badge]: https://img.shields.io/badge/Infrastructure-Kubernetes-informational?style=flat&logo=kubernetes&logoColor=white&color=316DE6
[license badge]: https://img.shields.io/badge/License-Apache%202.0-informational?style=flat&color=blue

[bash url]: https://www.gnu.org/software/bash
[hyperledger fabric url]: https://www.hyperledger.org/use/fabric
[kubernetes url]: https://kubernetes.io
[license url]: https://www.apache.org/licenses/LICENSE-2.0


[minikube]:  https://minikube.sigs.k8s.io/docs/
[helm]: https://helm.sh/

[crypto material]: https://hyperledger-fabric.readthedocs.io/en/release-2.2/identity/identity.html#digital-certificates
[network channel]: https://hyperledger-fabric.readthedocs.io/en/release-2.2/glossary.html#channel
[orderer service]: https://hyperledger-fabric.readthedocs.io/en/release-2.2/glossary.html#ordering-service
[blockchain peer]: https://hyperledger-fabric.readthedocs.io/en/release-2.2/glossary.html#peer
[chaincode]: https://hyperledger-fabric.readthedocs.io/en/release-2.2/glossary.html#smart-contract
[world state]: https://hyperledger-fabric.readthedocs.io/en/release-2.2/glossary.html#world-state
[couchdb]: https://hyperledger-fabric.readthedocs.io/en/release-2.2/couchdb_as_state_database.html
[kafka]: https://hyperledger-fabric.readthedocs.io/en/release-2.2/kafka.html
[raft]: https://hyperledger-fabric.readthedocs.io/en/release-2.2/orderer/ordering_service.html#raft
[golang]: https://github.com/golang/go
[gitlab ci]: https://about.gitlab.com/stages-devops-lifecycle/
[github actions]: https://github.com/features/actions
[hyperledger explorer]: https://www.hyperledger.org/use/explorer

[chainmetric contracts repo]: https://github.com/timoth-y/chainmetric-contracts
[chainmetric sensorsys repo]: https://github.com/timoth-y/chainmetric-network
[chainmetric app repo]: https://github.com/timoth-y/chainmetric-app

[license file]: https://github.com/timoth-y/chainmetric-network/blob/main/LICENSE
