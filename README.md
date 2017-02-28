# VNDR

[![Build Status](https://travis-ci.org/LK4D4/vndr.svg?branch=master)](https://travis-ci.org/LK4D4/vndr)

Vndr is simple vendoring tool, which is inspired by Docker vendor script.
Vndr has only two options: `-verbose` and `-whitelist`, both of which do
exactly what they say on the tin.

* `-verbose` adds additional output, helpful for debugging issues.
* `-keep-vcs-files` do not delete the vcs files.
* `-whitelist` allows you to specify several regular expressions for paths
  which will *not* be cleaned in the final stage of vendoring -- this is useful
  for running tests in a vendored project or otherwise ensuring that some
  important files are retained after `vndr` is done cleaning unused files from
  your `vendor/` directory.

## vendor.conf

`vendor.conf` is the configuration file of `vndr. It must have multiple lines,
which have format:
```
Import path               | revision                               | Repository(optional)
github.com/example/example 03a4d9dcf2f92eae8e90ed42aa2656f63fdd0b14 https://github.com/LK4D4/example.git

```
You can use `Repository` field for vendoring forks instead of original repos.
This config format is also accepted by [trash](https://github.com/rancher/trash).

## Initialization

You can initiate your project with vendor directory and `vendor.conf` using command
`vndr init`. This will populate your vendor directory with latest versions of
all dependencies and also write `vendor.conf` config which you can use for changing
versions later.

## Updating or using existing vendor.conf

If you already have `vendor.conf` you can just change versions there as you like,
set `$GOPATH` and run `vndr` in your repository with that file:
```
vndr
```
(Note: Your repository must be in proper place in `$GOPATH`, i.e. `$GOPATH/src/github.com/LK4D4/vndr`).

Also it's possible to vendor or update only one dependency:
```
vndr github.com/example/example 03a4d9dcf2f92eae8e90ed42aa2656f63fdd0b14 https://github.com/LK4D4/example.git
```
or
```
vndr github.com/example/example
```
to take revision and repo from `vendor.conf`.

If you experience any problems, please try to run `vndr -verbose`.

## Known issues

* `vndr` vendors only top packages, i.e. `golang.org/x/net` and not
`golang.org/x/net/trace`. If there is "non-top" packages in `vendor.conf`, `vndr`
might fail badly.
* if you use symlinks for managing `GOPATH` or packages in it, `vndr` might not
be able to work properly.
