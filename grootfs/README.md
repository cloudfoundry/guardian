# GrootFS: Garden root file system

[![slack.cloudfoundry.org](http://slack.cloudfoundry.org/badge.svg)](http://slack.cloudfoundry.org)

**Note:** This repository should be imported as `code.cloudfoundry.org/grootfs`.

![Groot](assets/groot.png)

[by](https://creativecommons.org/licenses/by-nc-nd/3.0/) [chattanooga-choochoo](http://chattanooga-choochoo.deviantart.com/art/Groot-584361210)

GrootFS is a [Cloud Foundry](https://www.cloudfoundry.org) component to satisfy
[garden-runc](https://github.com/cloudfoundry/garden-runc-release)'s
requirements for handling container images.

It is currently under development.

You can find us in the #garden [Cloud Foundry slack
channel](https://cloudfoundry.slack.com). Use
[https://slack.cloudfoundry.org](https://slack.cloudfoundry.org) to get an
invitation.


# Index
* [Installation](#installation)
* [Create a RootFS Image](#creating-a-rootfs-image)
* [Delete a RootFS Image](#deleting-a-rootfs-image)
* [Logging](#logging)
* [Clean up](#clean-up)
* [Known Issues](#known-issues)

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

_Using `go get code.cloudfoundry.org/grootfs` is discouraged because it might not work due to our versioned dependencies._

### Instructions

#### Requirements

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
  sudo btrfs quota enable /mnt/btrfs
  # you might need to chmod/chown the mount point if you don't want to run grootfs as root
  ```

* For user/group id mapping, you'll also require newuidmap and newgidmap to be
installed (uidmap package on ubuntu)

  ```
  sudo apt-get install uidmap
  ```


### Creating a RootFS Image

You can create a rootfs image based on a remote docker image:

```
grootfs --store /mnt/btrfs create docker:///ubuntu:latest my-image-id
```

Or from local folders as an image source:

```
grootfs --store /mnt/btrfs create /my-folder my-image-id
```


#### Output

The output of this command is a rootfs image path (`/mnt/btrfs/images/<uid>/my-image-id`) which has the following structure:

* The `<uid>` is the effective user id running the command.

```
<Returned directory>
|____ rootfs/
|____ image.json
```

* The `rootfs` folder is where the root filesystem lives.
* The `image.json` file follows the [OCI image description](https://github.com/opencontainers/image-spec/blob/master/serialization.md#image-json-description) schema.


#### User/Group ID Mapping

You might want to apply some user and group id mappings to the contents of the
`rootfs` folder. Grootfs supports the `--uid-mapping` and `--gid-mapping` arguments.
Suppose you are user with uid/gid 1000:

```
grootfs --store /mnt/btrfs create \
        --uid-mapping 0:1000:1 \
        --uid-mapping 1:100000:650000 \
        --gid-mapping 0:1000:1 \
        --gid-mapping 1:100000:650000 \
        docker:///ubuntu:latest \
        my-image-id
```

Some important notes:
* If you're not running as root, and you want to use mappings, you'll also need to map root (`0:--your-user-id:1`)
* Your id mappings can't overlap (e.g. 1:100000:65000 and 100:1000:200)
* You need to have these [mappings allowed](http://man7.org/linux/man-pages/man5/subuid.5.html) in the `/etc/subuid` and `/etc/subgid` files


#### Disk Quotas & Drax

Grootfs supports per-filesystem disk-quotas through the Drax binary.
BTRFS disk-quotas can only be enabled by a root user, therefore Drax must be owned by root, with the user bit set, and moved somewhere in the $PATH.

```
make
chown root drax
chmod u+s drax
mv drax /usr/local/bin/
```

Once Drax is configured, you can apply a quota to the rootfs:

```
grootfs --store /mnt/btrfs create \
        --disk-limit-size-bytes 10485760 \
        docker:///ubuntu:latest \
        my-image-id
```

### Deleting a RootFS Image

You can destroy a created rootfs image by calling `grootfs delete` with the image-id:

```
grootfs --store /mnt/btrfs delete my-image-id
```

Or the rootfs image path:

```
grootfs --store /mnt/btrfs delete /mnt/btrfs/images/<uid>/my-image-id
```

**Caveats:**

The store is based on the effective user running the command. If the user tries
to delete a rootfs image that does not belong to her/him the command fails.

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

### Clean up

```
grootfs --store /mnt/btrfs clean
```


When `clean` is called, any layers that aren't being used by a rootfs that
currently exists are deleted from the store\*.

For example: Imagine that we create two rootfs images from different base images, `Image
A` and `Image B`:

```
- Image A
  Layers:
    - layer-1
    - layer-2
    - layer-3

- Image B
  Layers:
    - layer-1
    - layer-4
    - layer-5

```

They have a layer in common, `layer-1`. And after deleting `Image B`,
`layer-4` and `layer-5` can be collected by `clean`, but not `layer-1` because
`Image A` still uses that layer.

It is safe to run the command in parallel, it does not interfere with other
creations or deletions.

The `clean` command has an optional integer parameter, `threshold-bytes`, and
when the store\* size is under that `clean` is a no-op, it does not remove
anything. On the other hand, if the store\* is over the threshold it cleans up
any resource that is not being used.  If 0 is provided it will behave the same
way as if the flag wasn't specified, it will clean up everything that's not
being used.  If a non integer or negative integer is provided, the command
fails without cleaning up anything.


**Caveats:**

The store is based on the effective user running the command. If the user tries
to clean up a store that does not belong to her/him the command fails.

\* It takes only into account the cache and volumes folders in the store.

## Misc

* All devices inside a image are ignored.

## Links

* [Garden project](https://github.com/cloudfoundry/garden)
* [GrootFS Pivotal tracker](https://www.pivotaltracker.com/n/projects/1661239)
* [GrootFS CI](https://grootfs.ci.cf-app.com)
* [Cloud Foundry Slack - Invitation](https://slack.cloudfoundry.org/)
* [Cloud Foundry Slack](https://cloudfoundry.slack.com/)

## Known Issues

#### Restrictions when running as a non-root user

* GrootFS cannot use `docker:///centos` when run as a non-root user without
  UID/GID mappings. CentOS sets the `/root` permissions to 0550 (i.e.:
  `r-xr-x---`) and therefore, when GrootFS runs as a non-root user it cannot
  write files into the rootfs `/root`. You can work around this by either
  running as root or [using a UID/GID mapping](#usergroup-id-mapping).

* Files not visible to the calling user in the base image won't be in the resulting rootfs.

* The store must have the right permissions or ownership for the calling user
  otherwise the command will fail.

* The calling user can only delete the rootfs that it owns.

* The calling user can only cleanup the cached volumes that it owns.

* The calling user can only request metrics for the rootfs that it owns.
