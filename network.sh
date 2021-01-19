#!/bin/bash

set -e

PROJECT_DIR=$PWD

ARGS_NUMBER="$#"
COMMAND="$1"

function verifyArg() {

    if [ $ARGS_NUMBER -ne 1 ]; then
        echo "Usage: networkOps.sh init | status | clean | cli | peer"
        exit 1;
    fi
}

OS_ARCH=$(echo "$(uname -s|tr '[:upper:]' '[:lower:]'|sed 's/mingw64_nt.*/windows/')-$(uname -m | sed 's/x86_64/amd64/g')" | awk '{print tolower($0)}')
FABRIC_ROOT=$GOPATH/src/github.com/hyperledger/fabric


function generateCryptoMaterials(){

    if [ -d ./crypto-config ]; then
            rm -rf ./crypto-config
    fi

    echo "> Generating certificates using cryptogen tool <"
    ./bin/cryptogen generate --config=./crypto-config.yaml

    echo
}


function generateChannelArtifacts(){

    if [ ! -d ./channel-artifacts ]; then
        mkdir channel-artifacts
    fi

    echo "> Generating channel configuration transaction 'channel.tx' <"
    ./bin/configtxgen -profile OrdererGenesis -channelID testchainid -outputBlock ./channel-artifacts/genesis.block
    ./bin/configtxgen -profile DeliveryChannel -outputCreateChannelTx ./channel-artifacts/channel.tx -channelID "delivery-channel"

    echo "> Generating anchor peer update for Supplier <"
    ./bin/configtxgen -profile DeliveryChannel -outputAnchorPeersUpdate ./channel-artifacts/supplierAnchors.tx  -channelID "delivery-channel" -asOrg supplierMSP


    echo "> Generating anchor peer update for Deliverer <"
    ./bin/configtxgen -profile DeliveryChannel -outputAnchorPeersUpdate ./channel-artifacts/delivererAnchors.tx  -channelID "delivery-channel" -asOrg delivererMSP

    echo
}

function startNetwork() {

    echo
    echo "================================================="
    echo "---------- Starting the network -----------------"
    echo "================================================="
    echo

    cd $PROJECT_DIR
    docker-compose -f docker-compose.yaml up -d
}

function cleanNetwork() {
    cd $PROJECT_DIR

    if [ -d ./channel-artifacts ]; then
            rm -rf ./channel-artifacts
    fi

    if [ -d ./crypto-config ]; then
            rm -rf ./crypto-config
    fi

    if [ -d ./tools ]; then
            rm -rf ./tools
    fi

    if [ -f ./docker-compose.yaml ]; then
        rm ./docker-compose.yaml
    fi

    if [ -f ./docker-compose.yamlt ]; then
        rm ./docker-compose.yamlt
    fi

    # This operations removes all docker containers and images regardless
    #
    docker rm -f $(docker ps -aq)
    docker rmi -f $(docker images -q)

    # This removes containers used to support the running chaincode.
    #docker rm -f $(docker ps --filter "name=dev" --filter "name=peer0.org1.example.com" --filter "name=cli" --filter "name=orderer.example.com" -q)

    # This removes only images hosting a running chaincode, and in this
    # particular case has the prefix dev-*
    #docker rmi $(docker images | grep dev | xargs -n 1 docker images --format "{{.ID}}" | xargs -n 1 docker rmi -f)
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
        generateCryptoMaterials
        generateChannelArtifacts
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
        echo "Usage: networkOps.sh init | status | clean | cli "
        exit 1;
esac