package groot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"

	"code.cloudfoundry.org/grootfs/store"
	errorspkg "github.com/pkg/errors"
)

const NamespaceFilename = "namespace.json"

type StoreNamespacer struct {
	storePath string
}

type mappings struct {
	UIDMappings []string `json:"uid-mappings"`
	GIDMappings []string `json:"gid-mappings"`
}

func NewStoreNamespacer(storePath string) *StoreNamespacer {
	return &StoreNamespacer{
		storePath: storePath,
	}
}

func (n *StoreNamespacer) Check(uidMappings, gidMappings []IDMappingSpec) (bool, error) {
	namespaceFilePath := n.namespaceFilePath()

	_, err := os.Stat(namespaceFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			if err := n.Write(n.storePath, uidMappings, gidMappings); err != nil {
				return false, err
			}

			return true, nil
		}
	}

	return n.validateNamespace(namespaceFilePath, uidMappings, gidMappings)
}

func (n *StoreNamespacer) Write(storePath string, uidMappings, gidMappings []IDMappingSpec) error {
	namespaceStore, err := os.Create(n.namespaceFilePath())
	if err != nil {
		return errorspkg.Wrap(err, "creating namespace file")
	}
	defer namespaceStore.Close()

	namespace := mappings{
		UIDMappings: n.normalizeMappings(uidMappings),
		GIDMappings: n.normalizeMappings(gidMappings),
	}

	if err := json.NewEncoder(namespaceStore).Encode(namespace); err != nil {
		return err
	}

	return nil
}

func (n *StoreNamespacer) validateNamespace(namespaceFilePath string, uidMappings, gidMappings []IDMappingSpec) (bool, error) {
	namespaceStore, err := os.Open(namespaceFilePath)
	if err != nil {
		return false, errorspkg.Wrap(err, "opening namespace file")
	}
	defer namespaceStore.Close()
	var namespace mappings
	if err := json.NewDecoder(namespaceStore).Decode(&namespace); err != nil {
		return false, errorspkg.Wrapf(err, "reading namespace file %s", namespaceStore.Name())
	}

	if !reflect.DeepEqual(namespace.UIDMappings, n.normalizeMappings(uidMappings)) {
		return false, nil
	}

	if !reflect.DeepEqual(namespace.GIDMappings, n.normalizeMappings(gidMappings)) {
		return false, nil
	}

	return true, nil
}

func (n *StoreNamespacer) namespaceFilePath() string {
	return filepath.Join(n.storePath, store.MetaDirName, NamespaceFilename)
}

func (n *StoreNamespacer) normalizeMappings(mappings []IDMappingSpec) []string {
	stringMappings := []string{}
	for _, mapping := range mappings {
		stringMappings = append(stringMappings, fmt.Sprintf("%d:%d:%d", mapping.NamespaceID, mapping.HostID, mapping.Size))
	}

	sort.Strings(stringMappings)
	return stringMappings
}
