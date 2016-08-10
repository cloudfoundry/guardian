## Concepts

## Image or repository

Is a set of layers that compose a file system which can be used by a container
as its root file system.

## Root file system

Is the `/` of a container.

## Layer or blob

Is a small fragment of a container image. It is usually produced as a file and
directory diff when running an operation (installing software etc) that writes
files and directories in an image.

## (Layer or blob) digest

Is an identifier (hash) which uniquely describes the contents of a layer.

## Registry

Is a server that stores and distributes images.

# Operation

## Clone

Is an operation which duplicates an image and applies the [U|G]ID mappings in
order to create a root file system.

## Fetch vs Stream

Fetch usually describes a blocking operation that transfers data somewhere in
the file system or memory (buffering). Stream is a similar term but applies
mostly to asynchronous transferring.
