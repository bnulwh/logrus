#!/bin/bash

set -e

# Install golanci 1.59.1
if [[ "$TRAVIS_GO_VERSION" =~ ^1\.21\. ]]; then
    curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s -- -b $(go env GOPATH)/bin v1.59.1
fi
