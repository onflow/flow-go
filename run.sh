#!/bin/bash
docker-compose up --build start_execute_dependencies && \
docker-compose up --build start_security_dependencies && \
docker-compose up --build execute && \
docker-compose up --build security
