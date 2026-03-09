# Contributing

Thank you for your interest in contributing to terraform-provider-cloudflareext!

## Developer Certificate of Origin (DCO)

All contributions to this project must be signed off in accordance with the [Developer Certificate of Origin (DCO)](https://developercertificate.org/).

The DCO is a lightweight way to certify that you wrote or otherwise have the right to submit the code you are contributing. The full text of the DCO is available at <https://developercertificate.org/> and is reproduced below:

```
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.


Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```

### How to sign off your commits

Add a `Signed-off-by` line to your commit messages:

```
Signed-off-by: Your Name <your.email@example.com>
```

You can do this automatically by using the `-s` or `--signoff` flag with `git commit`:

```bash
git commit -s -m "your commit message"
```

## How to Contribute

1. Fork the repository
2. Create a feature branch from `main`
3. Make your changes
4. Ensure tests pass: `make test`
5. Ensure code is formatted: `make fmt`
6. Ensure linting passes: `make lint`
7. Commit your changes with DCO sign-off: `git commit -s`
8. Push to your fork and submit a Pull Request

## Development Setup

### Prerequisites

- [Go](https://golang.org/doc/install) (see `go.mod` for version)
- [Terraform](https://www.terraform.io/downloads) >= 1.11
- [golangci-lint](https://golangci-lint.run/welcome/install/)

### Building

```bash
make build
```

### Testing

```bash
# Unit tests
make test

# Acceptance tests (requires Cloudflare credentials)
make testacc
```

### Local Installation

```bash
make install
```

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.
