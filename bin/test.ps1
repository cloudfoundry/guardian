$ErrorActionPreference = "Stop";
trap { $host.SetShouldExit(1) }

$env:GARDEN_TEST_ROOTFS = "N/A"
Invoke-Expression "go run github.com/onsi/ginkgo/v2/ginkgo $args --skip-package dadoo,gqt,kawasaki,locksmith,socket2me,signals,runcontainerd\nerd"
if ($LastExitCode -ne 0) {
  throw "tests failed"
}
Invoke-Expression "go run github.com/onsi/ginkgo/v2/ginkgo $args --skip-package dadoo,kawasaki,locksmith --focus 'Runtime Plugin' gqt"
if ($LastExitCode -ne 0) {
  throw "tests failed"
}
