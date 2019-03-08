package unpacker // import "code.cloudfoundry.org/grootfs/base_image_puller/unpacker"

import (
	"bytes"
	"encoding/json"
	"os"
	"strconv"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/sandbox"
	"code.cloudfoundry.org/lager"
	"github.com/containers/storage/pkg/reexec"
	errorspkg "github.com/pkg/errors"
)

type NSIdMapperUnpacker struct {
	shouldCloneUserNsOnUnpack bool
	storePath                 string
	reexecer                  groot.SandboxReexecer
	idMappings                groot.IDMappings
}

func init() {
	sandbox.Register("unpack", func(logger lager.Logger, extraFiles []*os.File, args ...string) error {
		if len(os.Args) != 6 {
			return errorspkg.New("wrong number of arguments")
		}

		targetDir := os.Args[1]
		baseDirectory := os.Args[2]
		uidMappingsJSON := os.Args[3]
		gidMappingsJSON := os.Args[4]
		shouldMapUidGid, err := strconv.ParseBool(os.Args[5])
		if err != nil {
			return errorspkg.Wrap(err, "parsing 'shouldMapUidGid' to bool")
		}

		if len(extraFiles) != 1 {
			return errorspkg.New("wrong number of extra files")
		}

		var uidMappings, gidMappings []groot.IDMappingSpec
		if err := json.Unmarshal([]byte(uidMappingsJSON), &uidMappings); err != nil {
			return errorspkg.Wrap(err, "unmarshaling uid mappings")
		}
		if err := json.Unmarshal([]byte(gidMappingsJSON), &gidMappings); err != nil {
			return errorspkg.Wrap(err, "unmarshaling gid mappings")
		}

		storeDir := extraFiles[0]
		whiteoutHandler := NewOverlayWhiteoutHandler(storeDir)

		idTranslator := NewNoopIDTranslator()
		if shouldMapUidGid {
			idTranslator = NewIDTranslator(uidMappings, gidMappings)
		}

		unpacker := NewTarUnpacker(whiteoutHandler, idTranslator)

		unpackOutput, err := unpacker.Unpack(logger, base_image_puller.UnpackSpec{
			Stream:        os.Stdin,
			TargetPath:    targetDir,
			BaseDirectory: baseDirectory,
		})
		if err != nil {
			return errorspkg.Wrap(err, "unpacking-failed")
		}

		if err := json.NewEncoder(os.Stdout).Encode(unpackOutput); err != nil {
			return errorspkg.Wrap(err, "encoding unpack output")
		}

		logger.Debug("unpack-command-ending")
		return nil
	})

	if reexec.Init() {
		// prevents infinite reexec loop
		// Details: https://medium.com/@teddyking/namespaces-in-go-reexec-3d1295b91af8
		os.Exit(0)
	}
}

func NewNSIdMapperUnpacker(storePath string, reexecer groot.SandboxReexecer, shouldCloneUserNsOnUnpack bool, idMappings groot.IDMappings) *NSIdMapperUnpacker {
	return &NSIdMapperUnpacker{
		shouldCloneUserNsOnUnpack: shouldCloneUserNsOnUnpack,
		storePath:                 storePath,
		reexecer:                  reexecer,
		idMappings:                idMappings,
	}
}

func (u *NSIdMapperUnpacker) Unpack(logger lager.Logger, spec base_image_puller.UnpackSpec) (base_image_puller.UnpackOutput, error) {
	logger = logger.Session("ns-id-mapper-unpacking", lager.Data{"spec": spec})
	logger.Debug("starting")
	defer logger.Debug("ending")

	uidMappingsJSON, err := json.Marshal(u.idMappings.UIDMappings)
	if err != nil {
		return base_image_puller.UnpackOutput{}, errorspkg.Wrap(err, "marshaling uid mappings")
	}

	gidMappingsJSON, err := json.Marshal(u.idMappings.GIDMappings)
	if err != nil {
		return base_image_puller.UnpackOutput{}, errorspkg.Wrap(err, "marshaling gid mappings")
	}

	shouldMapUidGid := strconv.FormatBool(!u.shouldCloneUserNsOnUnpack)
	out, err := u.reexecer.Reexec("unpack", groot.ReexecSpec{
		Stdin:       spec.Stream,
		ChrootDir:   spec.TargetPath,
		CloneUserns: u.shouldCloneUserNsOnUnpack,
		Args:        []string{".", spec.BaseDirectory, string(uidMappingsJSON), string(gidMappingsJSON), shouldMapUidGid},
		ExtraFiles:  []string{u.storePath},
	})
	if err != nil {
		return base_image_puller.UnpackOutput{}, errorspkg.Wrapf(err, "failed to unpack: %s", string(out))
	}

	var unpackOutput base_image_puller.UnpackOutput
	if err := json.NewDecoder(bytes.NewBuffer(out)).Decode(&unpackOutput); err != nil {
		return base_image_puller.UnpackOutput{}, errorspkg.Wrap(err, "invalid unpack output")
	}

	return unpackOutput, nil
}
