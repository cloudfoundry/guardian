package groot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/pkg/errors"

	"code.cloudfoundry.org/grootfs/store"
)

const NamespaceFilename = "namespace.json"

type StoreNamespaceChecker struct {
	storePath string
}

type mappings struct {
	UIDMappings []string `json:"uid-mappings"`
	GIDMappings []string `json:"gid-mappings"`
}

func NewNamespaceChecker(storePath string) *StoreNamespaceChecker {
	return &StoreNamespaceChecker{
		storePath: storePath,
	}
}

func (n *StoreNamespaceChecker) Check(uidMappings, gidMappings []IDMappingSpec) (bool, error) {
	namespaceStorePath := filepath.Join(n.storePath, store.MetaDirName, NamespaceFilename)

	_, err := os.Stat(namespaceStorePath)
	if err != nil {
		if os.IsNotExist(err) {
			return n.writeNamespace(namespaceStorePath, uidMappings, gidMappings)
		}
	}

	return n.validateNamespace(namespaceStorePath, uidMappings, gidMappings)
}

func (n *StoreNamespaceChecker) validateNamespace(namespaceFilePath string, uidMappings, gidMappings []IDMappingSpec) (bool, error) {
	namespaceStore, err := os.Open(namespaceFilePath)
	if err != nil {
		return false, fmt.Errorf("opening namespace file: %s", err)
	}
	defer namespaceStore.Close()
	var namespace mappings
	if err := json.NewDecoder(namespaceStore).Decode(&namespace); err != nil {
		return false, errors.Wrapf(err, "reading namespace file %s", namespaceStore)
	}

	if !reflect.DeepEqual(namespace.UIDMappings, n.normalizeMappings(uidMappings)) {
		return false, nil
	}

	if !reflect.DeepEqual(namespace.GIDMappings, n.normalizeMappings(gidMappings)) {
		return false, nil
	}

	return true, nil
}

func (n *StoreNamespaceChecker) writeNamespace(namespaceFilePath string, uidMappings, gidMappings []IDMappingSpec) (bool, error) {
	namespaceStore, err := os.Create(namespaceFilePath)
	if err != nil {
		return false, fmt.Errorf("creating namespace file: %s", err)
	}
	defer namespaceStore.Close()

	namespace := mappings{
		UIDMappings: n.normalizeMappings(uidMappings),
		GIDMappings: n.normalizeMappings(gidMappings),
	}

	if err := json.NewEncoder(namespaceStore).Encode(namespace); err != nil {
		return false, err
	}

	return true, nil
}

func (n *StoreNamespaceChecker) normalizeMappings(mappings []IDMappingSpec) []string {
	stringMappings := []string{}
	for _, mapping := range mappings {
		stringMappings = append(stringMappings, fmt.Sprintf("%d:%d:%d", mapping.NamespaceID, mapping.HostID, mapping.Size))
	}

	sort.Strings(stringMappings)
	return stringMappings
}
