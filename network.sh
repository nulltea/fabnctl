#!/bin/bash

set -e

PROJECT_DIR=$PWD

ARGS_NUMBER="$#"
COMMAND="$1"

function verifyArg() {

    if [ $ARGS_NUMBER -ne 1 ]; then
        echo "Usage: network.sh init | status | clean | cli | peer"
        exit 1;
    fi
}

OS_ARCH=$(echo "$(uname -s|tr '[:upper:]' '[:lower:]'|sed 's/mingw64_nt.*/windows/')-$(uname -m | sed 's/x86_64/amd64/g')" | awk '{print tolower($0)}')
FABRIC_ROOT=$GOPATH/src/github.com/hyperledger/fabric

function generateArtifacts(){
  helm upgrade --install artifacts charts/artifacts
}

function deployOrderer(){
  helm upgrade --install orderer charts/orderer
}

function deployPeers(){
  helm upgrade --install --set=config.mspID=supplierMSP,config.domain=supplier,config.peerSubdomain=peer0 peer0-supplier charts/peer-org
  helm upgrade --install --set=config.mspID=delivererMSP,config.domain=deliverer,config.peerSubdomain=peer0 peer0-deliverer charts/peer-org
}

function deployChannels() {
  peer channel create -o orderer:7050 -c supply-channel -f ./scripts/channel-artifacts/supply-channel.tx --tls true --cafile $ORDERER_CA
}

function enrollCA() {
  fabric-ca-client enroll -u https://admin:adminpw@localhost:7054 --caname=peer0-supplier-ca --tls.certfiles=/etc/hyperledger/fabric-ca-server-config/ca-cert.pem
  fabric-ca-client register -u https://admin:adminpw@localhost:7054 --caname=peer0-supplier-ca --id.name=supplieradmin --id.secret=supplieradminpw --id.type=admin --tls.certfiles=/etc/hyperledger/fabric-ca-server-config/ca-cert.pem
}

function cleanNetwork() {
    helm uninstall artifacts orderer peer0-supplier peer0-deliverer
    kubectl get pods -w
}

function networkStatus() {
    docker ps --format "{{.Names}}: {{.Status}}" | grep '[peer0* | orderer* | cli ]'
}

function dockerCli(){
    docker exec -it cli /bin/bash
}

# Network operations
verifyArg
case $COMMAND in
    "init")
        generateArtifacts
        ;;
    "deployOrderer")
        deployOrderer
        ;;
    "deployPeers")
        deployPeers
        ;;
    "status")
        networkStatus
        ;;
    "clean")
        cleanNetwork
        ;;
    "cli")
        dockerCli
        ;;
    *)
        echo "Usage: network.sh init | status | clean | cli "
        exit 1;
esac