# GrootFS: Garden root file system

[![slack.cloudfoundry.org](http://slack.cloudfoundry.org/badge.svg)](http://slack.cloudfoundry.org)

**Note:** This repository should be imported as `code.cloudfoundry.org/grootfs`.

![Groot](assets/groot.png)

[by](https://creativecommons.org/licenses/by-nc-nd/3.0/) [chattanooga-choochoo](http://chattanooga-choochoo.deviantart.com/art/Groot-584361210)

GrootFS is a [Cloud Foundry](https://www.cloudfoundry.org) component to satisfy
[garden-runc's](https://github.com/cloudfoundry/garden-runc-release)
requirements for handling container images.

It is currently under development.

You can find us in the #garden [Cloud Foundry slack
channel](https://cloudfoundry.slack.com). Use
[https://slack.cloudfoundry.org](https://slack.cloudfoundry.org) to get an
invitation.

## Installation

_Because grootfs depends on Linux kernel features, you can only build it from or
to a Linux machine._

```
mkdir -p $GOPATH/src/code.cloudfoundry.org
git clone https://github.com/cloudfoundry/grootfs.git $GOPATH/src/code.cloudfoundry.org/grootfs
cd $GOPATH/src/code.cloudfoundry.org/grootfs
git submodule update --init --recursive
make
```

_Using `go get code.cloudfoundry.org/grootfs` is discouraged because of the dependecies versions, it might not work_

## Instructions

### Requirements

* Grootfs requires btrfs to be enabled in the kernel, it also makes use of the brtfs-progs
(btrfs-tools package on ubuntu) for layering images.

  ```
  sudo apt-get install btrfs-tools
  sudo modprobe btrfs # if not loaded
  ```

* By default all operations will happen in `/var/lib/grootfs` folder, you can
change it by passing the `--store` flag to the binary. The store folder is expected
to be inside a mounted btrfs volume. If you don't have one, you can create a loop mounted
btrfs as follows:

  ```
  # create a btrfs block device
  truncate -s 1G ~/btrfs_volume
  mkfs.btrfs ~/btrfs_volume

  # mount the block device
  sudo mkdir -p /mnt/btrfs
  sudo mount -t btrfs -o user_subvol_rm_allowed ~/btrfs_volume /mnt/btrfs
  # you might need to chmod/chown the mount point if you'll not run grootfs as root
  ```

* For user/group id mapping, you'll also require newuidmap and newgidmap to be
installed (uidmap package on ubuntu)

  ```
  sudo apt-get install uidmap
  ```


### Creating a bundle

```
grootfs --store /mnt/btrfs create --image docker:///ubuntu:latest my-image-id
```

It also supports local folders as source of the image:

```
grootfs --store /mnt/btrfs create --image /my-folder my-image-id
```

This will create a `/mnt/btrfs/bundles/my-image-id/rootfs` directory with the
contents of `--image`.

#### User/Group ID Mapping

You might want to apply some user and group id mappings to the content of the
`rootfs` folder. Grootfs supports the `--uid-mapping` and `--gid-mapping` arguments.
Suppose you are user with uid/gid 1000:

```
grootfs --store /mnt/btrfs create \
        --uid-mapping 0:1000:1 \
        --uid-mapping 1:100000:650000 \
        --gid-mapping 0:1000:1 \
        --gid-mapping 1:100000:650000 \
        --image docker:///ubuntu:latest \
        my-image-id
```

Some important notes:
* If you're not running as root, and you want to use mappings, you'll always need to map root too (`0:--your-user-id:1`)
* Your id mappings can't overlap (e.g. 1:100000:65000 and 100:1000:200)
* You need to have these [mappings allowed](http://man7.org/linux/man-pages/man5/subuid.5.html) in the `/etc/subuid` and `/etc/subgid` files


### Deleting a bundle

You can destroy a created bundle/rootfs by calling grootfs with the image-id:

```
grootfs --store /mnt/btrfs delete my-image-id
```

### Logging

By default grootfs will not emit any logging, you can set the log level with the
`--log-level` flag:

```
grootfs --log-level debug create ...
```

It also supports redirecting the logs to a log file:

```
grootfs --log-level debug --log-file /var/log/grootfs.log create ...
```

## Links

* [Garden project](https://github.com/cloudfoundry/garden)
* [GrootFS Pivotal tracker](https://www.pivotaltracker.com/n/projects/1661239)
* [GrootFS CI](https://grootfs.ci.cf-app.com)
* [Cloud Foundry Slack - Invitation](https://slack.cloudfoundry.org/)
* [Cloud Foundry Slack](https://cloudfoundry.slack.com/)
