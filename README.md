# terraform-util

CLI for Terraform Registry documentation and provider helpers.

## Requirements

- Go 1.22 or newer

## Install

Install the latest released version into your Go bin directory:

```sh
go install github.com/sofushn/terraform-util-cli/cmd/terraform-util@latest
```

Install a specific version:

```sh
go install github.com/sofushn/terraform-util-cli/cmd/terraform-util@v0.2.0
```

Make sure your Go bin directory is on your `PATH`, then run:

```sh
terraform-util --help
```

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

## Install Local Checkout

To install the current checkout into your Go bin directory while developing:

```sh
go install ./cmd/terraform-util
```
