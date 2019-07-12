#!/bin/bash
docker-compose up --build start_collect_dependencies && \
docker-compose up --build start_consensus_dependencies && \
docker-compose up --build start_execute_dependencies && \
docker-compose up --build start_verify_dependencies && \
docker-compose up --build start_seal_dependencies && \
docker-compose up --build start_test_dependencies && \
docker-compose up --build test
