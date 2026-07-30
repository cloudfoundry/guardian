# Contributing to GrootFS

The GrootFS team uses GitHub and accepts contributions via [pull request](https://help.github.com/articles/using-pull-requests).

Please verify your changes before submitting a pull request by running the tests. See the [testing](https://github.com/cloudfoundry/grootfs#running-tests-in-concourse) section for more detail.

---

## Contributor License Agreement

Follow these steps to make a contribution to any of our open source repositories:

1. Ensure that you have completed our CLA Agreement for [individuals](https://www.cloudfoundry.org/wp-content/uploads/2015/07/CFF_Individual_CLA.pdf) or [corporations](https://www.cloudfoundry.org/wp-content/uploads/2015/07/CFF_Corporate_CLA.pdf).

1. Set your name and email (these should match the information on your submitted CLA)

  ```
  git config --global user.name "Firstname Lastname"
  git config --global user.email "your_email@example.com"
  ```

1. All contributions must be sent using GitHub pull requests as they create a nice audit trail and structured approach.

The originating github user has to either have a github id on-file with the list of approved users that have signed
the CLA or they can be a public "member" of a GitHub organization for a group that has signed the corporate CLA.
This enables the corporations to manage their users themselves instead of having to tell us when someone joins/leaves an organization. By removing a user from an organization's GitHub account, their new contributions are no longer approved because they are no longer covered under a CLA.

If a contribution is deemed to be covered by an existing CLA, then it is analyzed for engineering quality and product
fit before merging it.

If a contribution is not covered by the CLA, then the automated CLA system notifies the submitter politely that we
cannot identify their CLA and ask them to sign either an individual or corporate CLA. This happens automatially as a
comment on pull requests.

When the project receives a new CLA, it is recorded in the project records, the CLA is added to the database for the
automated system uses, then we manually make the Pull Request as having a CLA on-file.

---

## Development Environment

* Make sure you have golang >= 1.7 installed
* Clone GrootFS inside your GOPATH under `src/code.cloudfoundry.org/grootfs`.
* All dependencies are vendored as submodules (`git submodule update --init --recursive`)
* GrootFS only compiles in linux - it might be useful to set `GOOS=linux` if developing in a different platform
* Linux tests will fail locally if run on a different platform. To run these tests on anything other than Linux you'll need to [run the tests on Concourse](https://github.com/cloudfoundry/grootfs#running-tests-in-concourse).
* If you want to run tests locally you may optionally want to use [ginkgo](https://github.com/onsi/ginkgo). Otherwise you can use `go test`
	* To run a package test: `ginkgo ./<package-name>`
	* To run all tests locally: `ginkgo -r`

----

## Commit Style

We try to use the following template for git commit messages:

```
One-line description of your commit

A more verbose description of your commit if required. This
should include any context that will be useful to other developers
(or your future self)

[#tracker story id]
```

---

## Managing Go Dependencies

Our Go package dependencies are managed as git submodules in the `vendor` directory.
To add / remove / update dependencies, run the `script/deps` script as follows:

```
deps -a <url> --- add a new dependency
deps -d <url> --- remove a dependency
deps -u <url> --- update a dependency
deps -h --- print help menu
```
