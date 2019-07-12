#!/bin/bash
DIFF=$(git diff) 
if [ "$DIFF" != "" ]
then
    printf "Generated code is not up to date:\n\n$DIFF\n\n"
	exit 1
fi
