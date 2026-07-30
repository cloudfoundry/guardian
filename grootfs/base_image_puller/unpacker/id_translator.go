package unpacker

import "code.cloudfoundry.org/grootfs/groot"

//go:generate counterfeiter . IDTranslator
type IDTranslator interface {
	TranslateUID(id int) int
	TranslateGID(id int) int
}

type noopIDTranslator func(int) int

func NewNoopIDTranslator() IDTranslator {
	return noopIDTranslator(func(id int) int {
		return id
	})
}

func (n noopIDTranslator) TranslateUID(id int) int {
	return n(id)
}
func (n noopIDTranslator) TranslateGID(id int) int {
	return n(id)
}

type mappingIDTranslator struct {
	uidMappings []groot.IDMappingSpec
	gidMappings []groot.IDMappingSpec
}

func NewIDTranslator(uidMappings, gidMappings []groot.IDMappingSpec) IDTranslator {
	return &mappingIDTranslator{
		uidMappings: uidMappings,
		gidMappings: gidMappings,
	}
}

func (t *mappingIDTranslator) TranslateUID(id int) int {
	return translateID(id, t.uidMappings)
}
func (t *mappingIDTranslator) TranslateGID(id int) int {
	return translateID(id, t.gidMappings)
}

func translateID(id int, mappings []groot.IDMappingSpec) int {
	if id == 0 {
		return translateRootID(mappings)
	}

	for _, mapping := range mappings {
		if mapping.Size == 1 {
			continue
		}

		if id >= mapping.NamespaceID && id < mapping.NamespaceID+mapping.Size {
			return mapping.HostID + id - 1
		}
	}

	return id
}

func translateRootID(mappings []groot.IDMappingSpec) int {
	for _, mapping := range mappings {
		if mapping.Size == 1 {
			return mapping.HostID
		}
	}

	return 0
}
