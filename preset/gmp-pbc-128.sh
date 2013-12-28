#!/bin/bash 
cmake -DCHECK=off -DARITH=gmp -DFP_PRIME=254 -DFP_QNRES=on -DFP_METHD="BASIC;COMBA;COMBA;MONTY;LOWER;SLIDE" -DPP_METHD="INTEG;INTEG;LAZYR;OATEP" -DCOMP="-O2 -funroll-loops -fomit-frame-pointer" $1
