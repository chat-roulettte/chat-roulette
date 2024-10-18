#!/bin/bash

set -euo pipefail

POSTGRES_CONTAINER_NAME="postgres"
JAEGER_CONTAINER_NAME="jaeger"
GRAFANA_CONTAINER_NAME="grafana"

if [ "$#" -ne 1 ];
then
    echo "[-] Error: No positional arguments provided. Specify 'up' or 'down'"
    exit 1

elif [ "$1" == "up" ];
then
    echo "[*] Starting development environment..."

    echo "[*] Detected already running containers. Terminating them..."

    if [ "$(docker ps --filter "name=^${POSTGRES_CONTAINER_NAME}$" --format '{{.Names}}')" == $POSTGRES_CONTAINER_NAME ]
    then
        make -s db/stop
    fi

    if [ "$(docker ps --filter "name=^${JAEGER_CONTAINER_NAME}$" --format '{{.Names}}')" == $JAEGER_CONTAINER_NAME ]
    then
        docker rm -f "${JAEGER_CONTAINER_NAME}"
    fi

    if [ "$(docker ps --filter "name=^${GRAFANA_CONTAINER_NAME}$" --format '{{.Names}}')" == $GRAFANA_CONTAINER_NAME ]
    then
        docker rm -f "${GRAFANA_CONTAINER_NAME}"
    fi

    make -s db/run

    echo -n "[*] Waiting for postgres to be ready..."

    while ! (docker logs $POSTGRES_CONTAINER_NAME 2>&1 | grep -Pzlq '(?s)init process complete.*\n.*ready to accept connections')
    do
        echo -n "."
        sleep 1
    done

    echo
    echo "[*] Running database migration..."

    make -s migrate/up

    echo "[*] Running jaeger and grafana"
    docker run -d -p 16686:16686 -p 4317:4317 -p 4318:4318 --name "${JAEGER_CONTAINER_NAME}" docker.io/jaegertracing/all-in-one:1.52
    docker run -d -p 3000:3000 --name="${GRAFANA_CONTAINER_NAME}" docker.io/grafana/grafana:11.1.3

    make -s go/run

elif [ "$1" == "destroy" ];
then
    echo "[*] Stopping development environment"
    make db/stop
    docker rm -f "${JAEGER_CONTAINER_NAME}"
    docker rm -f "${GRAFANA_CONTAINER_NAME}"
fi
