package groot

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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

func (n *StoreNamespacer) ApplyMappings(uidMappings, gidMappings []IDMappingSpec) error {
	namespaceFilePath := n.namespaceFilePath()

	_, err := os.Stat(namespaceFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return n.write(uidMappings, gidMappings)
		}
	}

	return n.validateNamespace(namespaceFilePath, uidMappings, gidMappings)
}

func (n *StoreNamespacer) Read() (IDMappings, error) {
	mappingsFromFile := mappings{}
	jsonBytes, err := ioutil.ReadFile(n.namespaceFilePath())
	if err != nil {
		return IDMappings{}, errorspkg.Wrap(err, "reading namespace file")
	}
	if err := json.Unmarshal(jsonBytes, &mappingsFromFile); err != nil {
		return IDMappings{}, errorspkg.Wrap(err, "invalid namespace file")
	}

	uidMappings, err := n.parseIDMappings(mappingsFromFile.UIDMappings)
	if err != nil {
		return IDMappings{}, errorspkg.Wrap(err, "invalid uid mappings format")
	}

	gidMappings, err := n.parseIDMappings(mappingsFromFile.GIDMappings)
	if err != nil {
		return IDMappings{}, errorspkg.Wrap(err, "invalid gid mappings format")
	}

	return IDMappings{
		UIDMappings: uidMappings,
		GIDMappings: gidMappings,
	}, nil
}

func (n *StoreNamespacer) write(uidMappings, gidMappings []IDMappingSpec) error {
	namespaceStore, err := os.Create(n.namespaceFilePath())
	if err != nil {
		return errorspkg.Wrap(err, "creating namespace file")
	}
	defer namespaceStore.Close()

	namespace := mappings{
		UIDMappings: n.normalizeMappings(uidMappings),
		GIDMappings: n.normalizeMappings(gidMappings),
	}

	if err := os.Chmod(namespaceStore.Name(), 0755); err != nil {
		return errorspkg.Wrap(err, "failed to chmod namespace file")
	}
	return json.NewEncoder(namespaceStore).Encode(namespace)
}

func (n *StoreNamespacer) validateNamespace(namespaceFilePath string, uidMappings, gidMappings []IDMappingSpec) error {
	namespaceStore, err := os.Open(namespaceFilePath)
	if err != nil {
		return errorspkg.Wrap(err, "opening namespace file")
	}
	defer namespaceStore.Close()
	var namespace mappings
	if err := json.NewDecoder(namespaceStore).Decode(&namespace); err != nil {
		return errorspkg.Wrapf(err, "reading namespace file %s", namespaceStore.Name())
	}

	if !reflect.DeepEqual(namespace.UIDMappings, n.normalizeMappings(uidMappings)) {
		return errorspkg.New("provided UID mappings do not match those already configured in the store")
	}

	if !reflect.DeepEqual(namespace.GIDMappings, n.normalizeMappings(gidMappings)) {
		return errorspkg.New("provided GID mappings do not match those already configured in the store")
	}

	return nil
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

func (n *StoreNamespacer) parseIDMappings(args []string) ([]IDMappingSpec, error) {
	mappings := []IDMappingSpec{}

	for _, v := range args {
		var mapping IDMappingSpec
		_, err := fmt.Sscanf(v, "%d:%d:%d", &mapping.NamespaceID, &mapping.HostID, &mapping.Size)
		if err != nil {
			return nil, err
		}
		mappings = append(mappings, mapping)
	}

	return mappings, nil
}
