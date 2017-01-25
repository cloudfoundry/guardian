package main

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"code.cloudfoundry.org/guardian/bindata"
	"code.cloudfoundry.org/guardian/guardiancmd"
	"github.com/jessevdk/go-flags"
)

func main() {
	cmd := &guardiancmd.GuardianCommand{}

	parser := flags.NewParser(cmd, flags.Default)
	parser.NamespaceDelimiter = "-"

	args, err := parser.Parse()
	if err != nil {
		os.Exit(1)
	}

	// gdn can be compiled for one of two possible run "modes"
	// 1. all-in-one    - this is meant for standalone deployments
	// 2. bosh-deployed - this is meant for deployment via BOSH
	// when compiling an all-in-one gdn, the bindata package will contain a
	// number of compiled assets (e.g. iptables, runc, etc.), thus we check to
	// see if we have any compiled assets here and perform additional setup
	// (e.g. updating bin paths to point to the compiled assets) if required
	if len(bindata.AssetNames()) > 0 {
		err := checkRoot()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		depotDir := cmd.Containers.Dir
		err = os.MkdirAll(depotDir, 0755)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		restoredAssetsDir, err := restoreUnversionedAssets(cmd.Bin.AssetsDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		cmd.Bin.Runc = filepath.Join(restoredAssetsDir, "bin", "runc")
		cmd.Bin.Dadoo = guardiancmd.FileFlag(filepath.Join(restoredAssetsDir, "bin", "dadoo"))
		cmd.Bin.Init = guardiancmd.FileFlag(filepath.Join(restoredAssetsDir, "bin", "init"))
		cmd.Bin.NSTar = guardiancmd.FileFlag(filepath.Join(restoredAssetsDir, "bin", "nstar"))
		cmd.Bin.Tar = guardiancmd.FileFlag(filepath.Join(restoredAssetsDir, "bin", "tar"))
		cmd.Bin.IPTables = guardiancmd.FileFlag(filepath.Join(restoredAssetsDir, "sbin", "iptables"))
		cmd.Bin.IPTablesRestore = guardiancmd.FileFlag(filepath.Join(restoredAssetsDir, "sbin", "iptables-restore"))

		cmd.Network.AllowHostAccess = true
	}

	err = cmd.Execute(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func checkRoot() error {
	currentUser, err := user.Current()
	if err != nil {
		return err
	}

	if currentUser.Uid != "0" {
		return errors.New("server must be run as root")
	}

	return nil
}

func restoreUnversionedAssets(assetsDir string) (string, error) {
	okMarker := filepath.Join(assetsDir, "ok")

	_, err := os.Stat(okMarker)
	if err == nil {
		return "", nil
	}

	err = bindata.RestoreAssets(assetsDir, "linux")
	if err != nil {
		return "", nil
	}

	ok, err := os.Create(okMarker)
	if err != nil {
		return "", nil
	}

	err = ok.Close()
	if err != nil {
		return "", nil
	}

	return filepath.Join(assetsDir, "linux"), nil
}
