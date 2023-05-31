#!/bin/bash

# keeps track of all the historical times TPS tests were run
date +"Current date and time is %a %b %d %T %Z %Y" | tee -a /opt/runs.txt
