# Guardian

**Note**: This repository should be imported as `code.cloudfoundry.org/guardian`.

A simple single-host OCI container manager.

## Developing and Deploying Guardian

For details on how to get started with developing or deploying Guardian please check out the
[Garden-runC release repo](https://github.com/cloudfoundry/garden-runc-release/blob/master/README.md)

## Components

 - **Gardeners Question Time (GQT):** A venerable British radio programme. And also a test suite.
 - **Gardener:** Orchestrates the other components. Implements the Cloud Foundry Garden API. 
 - **[Garden Shed](http://github.com/cloudfoundry/garden-shed):** RootFS and volume management. Where stuff is kept in the garden.
 - **RunDMC:** A tiny wrappper around RunC to manage a collection of RunC containers.
 - **Kawasaki:** It's an amazing networker.
