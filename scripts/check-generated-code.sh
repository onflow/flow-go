#!/bin/bash
if [[ $(git status --porcelain) ]]
then
    printf "Generated code is not up to date"
	exit 1
fi
