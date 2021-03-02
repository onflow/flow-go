#!/usr/bin/env bash
docker_image="dapperlabs/plantuml"
host_src_volume="${PWD}"
container_dest_volume="/work"

_generate_by_type() {
    docker run --volume="${host_src_volume}:${container_dest_volume}" --workdir="${container_dest_volume}" --rm --attach=stdin --attach=stdout --interactive "${docker_image}" -"$1" -charset utf-8 **/*.uml
}

_print() {
    echo ""
    echo "$1"
    echo "-----------------------------"
}

_print "Building plantuml Docker Image"
docker build . -t ${docker_image}

_print "Generating SVGs"
_generate_by_type "tsvg"


_print "Generating PNGs"
_generate_by_type "tpng"
