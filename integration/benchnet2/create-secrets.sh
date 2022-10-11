#!/bin/bash

# Create execution-state secrets required to run network
# Note - As K8s secrets cannot contain forward slashes, we remove the path prefix
# Note - Since this is non-secret, this could be a configmap rather than a secret
for f in bootstrap/execution-state/*; do
    # Remove the bootstrap/execution-state/ prefix
    # Example start bootstrap/execution-state/00000000
    # Example result 00000000
    PREFIXREMOVED=${f//bootstrap\/execution-state\//};

    # Create the secret after string manipulation
    kubectl create secret generic $PREFIXREMOVED --from-file=$f;
    kubectl label secret $PREFIXREMOVED "service=flow"
done

# Create private-root-information secrets required to run network
# Note - As K8s secrets cannot contain forward slashes, the "${PREFIXREMOVED///\//.}" replaces forward slashes with periods
# Example filename bootstrap/private-root-information/private-node-info_416c65782048656e74736368656c00e4e3235298a4b91382ecd84f13b9c237e6/node-info.priv.json
# Example key name result after string manipulation 416c65782048656e74736368656c00e4e3235298a4b91382ecd84f13b9c237e6.node-info.priv.json
for f in bootstrap/private-root-information/*/*; do
    # Remove the bootstrap/private-root-information/private-node-info_ prefix to ensure NodeId is retained
    # Example result 416c65782048656e74736368656c00e4e3235298a4b91382ecd84f13b9c237e6/node-info.priv.json
    PREFIXREMOVED=${f//bootstrap\/private-root-information\/private-node-info_/};

    # Substitute the forward slash "/" for a period "."
    # Example $PREFIXREMOVED value 416c65782048656e74736368656c00e4e3235298a4b91382ecd84f13b9c237e6/node-info.priv.json
    # Example result after string manipulation 416c65782048656e74736368656c00e4e3235298a4b91382ecd84f13b9c237e6.node-info.priv.json
    KEYNAME=${PREFIXREMOVED//\//.}
    
    # Create the secret after string manipulation
    kubectl create secret generic $KEYNAME --from-file=$f;
    kubectl label secret $KEYNAME "service=flow"
done

# Create public-root-information secrets required to run network
# Note - As K8s secrets cannot contain forward slashes, we remove the path prefix
# Note - Since this is non-secret, this could be a configmap rather than a secret
for f in bootstrap/public-root-information/*.json; do
    # Remove the bootstrap/public-root-informationn/private-node-info_ prefix
    # Example start bootstrap/public-root-information/node-infos.pub.json
    # Example result node-info.pub.json
    PREFIXREMOVED=${f//bootstrap\/public-root-information\//};

    # Create the secret after string manipulation
    kubectl create secret generic $PREFIXREMOVED --from-file=$f ; 
    kubectl label secret $PREFIXREMOVED "service=flow"
done
