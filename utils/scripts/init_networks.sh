#!/bin/bash

CWD=$(pwd)

trap "finish" INT TERM

finish() {
    local existcode=$?
    exit $existcode
}

API_NETWORK="atai_apis_network"
API_NETWORK_INSPECTION=$(docker network inspect $API_NETWORK)
API_NETWORK_INSPECTION=$?

if [ $API_NETWORK_INSPECTION -ne 0 ]
then
    echo "Creating $API_NETWORK network... "
    docker network create \
        --driver="bridge" \
        --subnet="173.11.0.0/24" \
        --gateway="173.11.0.1" \
        $API_NETWORK
else
    echo "$API_NETWORK already exists"
fi

CONTROL_MECHANISM_NETWORK="atai_control_mechanism"
CONTROL_MECHANISM_NETWORK_INSPECTION=$(docker network inspect $CONTROL_MECHANISM_NETWORK)
CONTROL_MECHANISM_NETWORK_INSPECTION=$?

if [ $CONTROL_MECHANISM_NETWORK_INSPECTION -ne 0 ]
then
    echo "Creating $CONTROL_MECHANISM_NETWORK network..."
    docker network create \
        --driver="bridge" \
        --subnet="173.10.0.0/24" \
        --gateway="173.10.0.1" \
        $CONTROL_MECHANISM_NETWORK
else
    echo "$CONTROL_MECHANISM_NETWORK already exists"
fi
