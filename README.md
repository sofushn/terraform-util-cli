# terraform-registry-cli

CLI for Terraform Registry documentation and provider helpers.

## Requirements

- Go 1.22 or newer

## Build

From the repository root:

```sh
go build -o terraform-registry ./cmd/terraform-registry
```

Run the built CLI:

```sh
./terraform-registry --help
```

## Try the CLI

```sh
./terraform-registry search aws
./terraform-registry add aws
./terraform-registry remove aws
./terraform-registry update aws
./terraform-registry update aws --version "~> 6.1"
./terraform-registry docs list aws
./terraform-registry docs list aws vpc
./terraform-registry docs aws resource/aws_vpc
```

## Test

```sh
go test ./...
```

## Install Locally

To install the binary into your Go bin directory:

```sh
go install ./cmd/terraform-registry
```

Make sure your Go bin directory is on your `PATH`, then run:

```sh
terraform-registry --help
```
