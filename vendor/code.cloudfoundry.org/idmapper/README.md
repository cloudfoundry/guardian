# IdMapper

[![GoDoc](https://godoc.org/code.cloudfoundry.org/idmapper?status.svg)](https://godoc.org/code.cloudfoundry.org/idmapper)

idmapper is a package which will map a process to the highest user id available.
It was created to be used by [GrootFS](https://github.com/cloudfoundry/grootfs#grootfs-garden-root-file-system), a root filesystem manager for [CloudFoundry](https://docs.cloudfoundry.org/)'s container runtime.

Unlike the `newuidmap` and `newgidmap` commands found in [Shadow](http://pkg-shadow.alioth.debian.org/), idmapper does not require this user to exist and will not check `/etc/subuid` for valid subuid ranges.

## Commands
### `newuidmap` / `newgidmap`
Will map the given process to the maximum user id available
e.g.
```
$ newuidmap <process id>
$ newgidmap <process id>
```
### `maximus`
Will return the maximum user id available.
```
$ maximus
# => 4294967294
```

## `idmapper` package
This can be used by importing:
```
"code.cloudfoundry.org/idmapper"
```
