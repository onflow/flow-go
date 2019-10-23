#!/bin/bash
IFS=
status="$(git status --porcelain)"
if [[ $status ]]
then
    echo "Generated code is not up to date:"
    echo $status
    exit 1
fi
