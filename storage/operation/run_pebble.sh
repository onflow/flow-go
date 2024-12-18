#!/bin/bash

# Define the loop count
MAX_RUNS=100

# Function to clean directories and run tests
for i in $(seq 1 $MAX_RUNS); do
    echo "============================"
    echo "Run #$i"
    echo "============================"

    # Step 1: Clean directories
    echo "Step 1: Cleaning directories..."
    rm -rf pebbletest/* badgertest/*

    # Step 2: Run TestInsertApprovalDurability/Pebble (ignore errors)
    echo "Step 2: Running TestInsertApprovalDurability/Pebble (ignoring errors)..."
    go test --failfast -run=TestInsertApprovalDurability/Pebble -v || true

    # Step 3: Run TestReadApproval/Pebble
    echo "Step 3: Running TestReadApproval/Pebble..."
    go test --failfast -run=TestReadApproval/Pebble -v
    if [ $? -ne 0 ]; then
        echo "Step 3 failed. Exiting..."
        exit 1
    fi

    echo "Run #$i completed successfully."
done

echo "All $MAX_RUNS runs completed successfully!"
