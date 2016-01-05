# VNDR

Vndr is simple vendoring tool, which is inspired by docker vendor script.
It has only two options: init new vendor and update it.
You can init your repo with config and vendor dir by:
```
vndr init
```
and update after modifying (change revision or add new dependency) `vndr.cfg`
with just
```
vndr
```

It downloads all dependencies to `vendor/` directory. It uses new vendor layout
from `go1.6` (GO15VENDOREXPERIMENT) and also relies on `go1.6` features. So,
you need at least `go1.6beta1` to compile `vndr` and `GO15VENDOREXPERIMENT=1` in
`go1.5` to compile your project.
