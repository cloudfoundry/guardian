# oci-test-image

A collection of OCI images (some valid, some invalid) for use in grootfs's integration tests.

## About

Most of these images are handcrafted to meet the needs of a test.

### noparents

This image ensures that grootfs is correctly able to handle images in which parent dirs do not exist as entries in a layer's .tar file. This test image was built using a combination of busybox, skopeo and tar. The base image is busybox. A few additional directories, links and files have been added for use in testing, specifically:

```
a/path/to/cakes/cake.txt
another/path/to/cakes
hardlinks/a
hardlinks/b
softlinks/a
softlinks/b
```

Non-default permissions have been applied to some of the dirs, specifically `a/path/to/cakes` and `another/path/to/cakes` both have 777 permissions. `cake.txt` has been chowned to uid:gid 1000:1000 with 0600 perms. The command to build the .tar file used in the oci-image is as follows:

```
tar -cf noparents.tar a/path/to/cakes/cake.txt another/path/to/cakes/ hardlinks/a hardlinks/b softlinks/a softlinks/b bin/ dev/ etc/ home/ lib/ lib64 root/ tmp/ usr/ var/
```

Which results in a tar file that looks like this:


```
a/path/to/cakes/cake.txt
another/path/to/cakes/
hardlinks/a
hardlinks/b
softlinks/a
softlinks/b
bin/
...
```

Which is what is required for the test.
