# Contributing Guidelines

This document describes how to contribute to the project.

## Prerequisites

- Go version 1.20+
- A valid OpenShift Cluster Manager (OCM) token and logged in via [ocm-cli](https://github.com/openshift-online/ocm-cli) or [rosa](https://github.com/openshift/rosa)
- Access to ocm-backplane

## Contributing steps

1. Submit an issue describing your proposed change
2. Fork the desired repo, develop and test your code changes
3. Submit a pull request

## What to do before submitting a pull request

1. Ensure any additional AWS permissions are available in the [STS Support Policy](https://github.com/openshift/managed-cluster-config/blob/master/resources/sts/4.11/sts_support_permission_policy.json)

2. Ensure the project builds and all tests pass

    ```shell
    go test -v ./...
    ```

3. Test the change against a staging ROSA cluster
    
    ```shell
    ocm-backplane tunnel -D
    mirrosa -cluster-id "${CLUSTER_ID}"
    ```

## New Releases

New releases are currently manually triggered via [.github/workflows/release.yml](.github/workflows/release.yml) and produce binaries and [cosign](https://github.com/sigstores/cosign) to sign them.
