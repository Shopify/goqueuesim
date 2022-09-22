# Contributing

Thank you for considering contributing to goqueuesim!

## Getting Started

- Review this document and the [Code of Conduct](CODE_OF_CONDUCT.md).

- Setup a [Go development environment](https://golang.org/doc/install#install)
if you haven't already.

- Get the Shopify version the project by using Go get:

```shell
$ go get -u github.com/Shopify/goqueuesim/cmd/goqueuesim
```

- Fork this project on GitHub. :octocat:

- Setup your fork as a remote for your project:

```
$ cd $GOPATH/src/github.com/Shopify/goqueuesim
$ git remote add <your username> <your fork's remote path>
```


## Work on your feature

- Create your feature branch based off of the `main` branch. (It might be
worth doing a `git pull` if you haven't done one in a while.)

```
$ git checkout main
$ git pull
$ git checkout -b <the name of your branch>
```

- Code/write! :keyboard:

    - If working on code, please run `go fmt` and `golangci-lint` before opening a PR against `main`.

- Push your changes to your fork's remote:

```
$ git push -u <your username> <the name of your branch>
```

## Send in your changes

- Sign the [Contributor License Agreement](https://cla.shopify.com).

- Open a PR against Shopify/goqueuesim!


## Releasing

Binaries and packages are built with [GoReleaser](https://goreleaser.com). GoReleaser runs when a new version is tagged.

This project uses
[Semantic Versioning](https://semver.org) which basically means that API
compatible changes should bump the last digit, backwards compatible changes
should bump the second, and API incompatible changes should bump the first
digit. For example, if we are fixing a bug that doesn't affect other systems
we can bump from v1.0.0 to v1.0.1. If the change is backwards compatible with
the previous version, we can bump from v1.0.0 to v1.1.0. If the change is
not backwards compatible, we must bump the version from v1.0.0 to v2.0.0.

Run the following, where `version` is replaced with the appropriate version for
this release.

(Note that you will need to have Git configured to sign tags with
your OpenPGP key or this command will fail.)

```shell
$ git tag -s <version>
```

Before pushing your tag, build a release version of goqueuesim to ensure that
everything builds properly:

```shell
$ make release
```

If this step fails, please do not make a release without fixing it. In
addition, please delete the tag so it can be replaced once the issue is
fixed.

Next, push the tag to the server, where `version` is the same version you
specified before:

```shell
$ git push origin refs/tags/<version>
```

Finally, create a new release in GitHub for the new version for the tag you
created and signed. In the `dist/` directory you will find the automatically
generated binary tar archives and `checksums.txt` file, which will need to
be added to the release in GitHub.

Please update `CHANGELOG.md` to include a short-form description of your changes.

Finally, bump `/VERSION`, commit the changes and open a PR to merge the changes into `main`.
