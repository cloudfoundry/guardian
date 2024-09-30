## Concepts

## Image

Is the set of layers that compose a file system which can be used by a container
as its root file system.

## Root file system (rootfs)

Is the filesystem mounted at `/` of a container.

## Layer blob

Is a small fragment of a container image. It is usually produced as a file and
directory diff when running an operation (installing software etc) that writes
files and directories in an image.

## Config blob

A special blob which contains metadata about an image, rather than root filesystem
fragments. Usually a JSON document.

## (Layer or blob) digest

Is an identifier (hash) which uniquely identifies a layer by its content. In the
usual case this is the SHA256 sum of the *compressed* layer tarfile. When GrootFS
fetches layer blobs from a registry, their digests are computed and checked against
those stored in the image manifest. The addressing of blobs by content (by way of their
digest) is an essential component of container filesystem security, preventing layer
cache poisoning attacks.

## Diff ID

Is another identifier (hash) which uniquely identifies a layer by its contents. It
differs from the layer digest in that it is strictly the SHA256 of the *uncompressed*
layer tarfile.

## Chain ID

An identifier which uniquely identifies a stack of layers. In the general case, the
chain ID of a layer is the SHA256 sum of a layer's diff ID concatenated with the chain
ID of the layer's parent layer.

## Docker image

An image which conforms to one of the Docker image formats, typically stored in a Docker registry.
GrootFS is capable of fetching Docker images and extracting them to provide container filesystems.
Docker images are located with a URL of the form `docker://[registry]/repository/image`, where the
optional registry component defaults to [Docker Hub](https://hub.docker.com/).

## Docker V2 Schema 1 image

An image, stored in a Docker V2 registry, with a manifest which conforms to the
[Docker manifest schema 1](https://docker-docs.uclv.cu/registry/spec/manifest-v2-1/).
This manifest format is deprecated, but GrootFS is still capable of fetching and utilising these images.
Internally, this is done by converting the image to Schema 2 format once fetched. This is an expensive
process since Schema 1 manifests do not contain diff IDs, and so layers must be fetched in order for these
to be computed.

## Docker V2 Schema 2 image

An image, stored in a Docker V2 registry, with a manifest which conforms to the
[Docker manifest schema 1](https://docker-docs.uclv.cu/registry/spec/manifest-v2-2/).
This is the manifest format currently used to describe Docker images generated with modern Docker tooling.

## OCI image

An image which conforms to the [OCI Image Format Specification](https://github.com/opencontainers/image-spec). GrootFS is capable of
unpacking local OCI images and extracting them to provide container filesystems. OCI images are
located with a URL of the form `oci://path/to/oci/image`.

## Tarfile

Image layers are typically shipped around as [tar files](https://en.wikipedia.org/wiki/Tar_(computing)), usually compressed for transport across
the network. GrootFS is also capable of unpacking arbitrary tar files for use as container root
filesystems. Tar files for use in this way are located with a URL of the form `/path/to/file.tar`.
Tar files are unpacked ready for use into GrootFS's store.

## Registry

Is a server that stores and distributes images. Fetching an image from a registry involves
first fetching the metadata that describes the image, and then walking through this metadata
to fetch and store the config and layer blobs that it describes.

## Docker V2 Registry

Currently the only registry type supported by GrootFS is the [Docker Registry HTTP API V2](https://docker-docs.uclv.cu/registry/spec/api/). The Docker V1 registry
API has long since been deprecated and is not supported by GrootFS.

## Store

The directory that contains the rootfs images and cached layers.

# Operations

## Clone

Is an operation which duplicates an image and applies the [U|G]ID mappings in
order to create a root file system.

## Fetch vs Stream

Fetch usually describes a blocking operation that transfers data somewhere in
the file system or memory (buffering). Stream is a similar term but applies
mostly to asynchronous transferring.
