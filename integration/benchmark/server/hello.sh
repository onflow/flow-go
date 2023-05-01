#!/bin/bash

date +"Hello. The current date and time is %a %b %d %T %Z %Y" | tee -a /opt/hello.txt
