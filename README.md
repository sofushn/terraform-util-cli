# terraform-util

CLI for Terraform Registry documentation and provider helpers.

## Requirements

- Go 1.22 or newer

## Build

From the repository root:

```sh
go build -o terraform-util ./cmd/terraform-util
```

Run the built CLI:

```sh
./terraform-util --help
```

## Try the CLI

```sh
./terraform-util search aws
./terraform-util add aws
./terraform-util remove aws
./terraform-util update aws
./terraform-util update aws --version "~> 6.1"
./terraform-util docs list aws
./terraform-util docs list aws vpc
./terraform-util docs aws resource/aws_vpc
```

## Test

```sh
go test ./...
```

## Install Locally

To install the binary into your Go bin directory:

```sh
go install ./cmd/terraform-util
```

Make sure your Go bin directory is on your `PATH`, then run:

```sh
terraform-util --help
```
