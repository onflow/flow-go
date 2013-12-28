#!/bin/bash 
cmake -DCHECK=on -DSEED=ZERO -DALLOC=DYNAMIC -DDEBUG=on -DSHLIB=off -DSIMUL="/usr/bin/valgrind" -DSIMAR="--tool=memcheck --track-origins=yes --leak-check=full" $1
