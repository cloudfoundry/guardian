package source // import "code.cloudfoundry.org/grootfs/fetcher/layer_fetcher/source"

import (
	"context"
	"net/url"

	"code.cloudfoundry.org/lager"
	"github.com/containers/image/transports"
	"github.com/containers/image/types"
	errorspkg "github.com/pkg/errors"
)

//go:generate counterfeiter . ImageSource
type ImageSource interface {
	types.ImageSource
}

func CreateImageSource(logger lager.Logger, systemContext types.SystemContext, baseImageURL *url.URL) (types.ImageSource, error) {
	ref, err := reference(logger, baseImageURL)
	if err != nil {
		return nil, err
	}

	imgSrc, err := ref.NewImageSource(context.TODO(), &systemContext)
	if err != nil {
		return nil, errorspkg.Wrap(err, "creating image source")
	}

	return imgSrc, nil
}

func reference(logger lager.Logger, baseImageURL *url.URL) (types.ImageReference, error) {
	refString := "/"
	if baseImageURL.Host != "" {
		refString += "/" + baseImageURL.Host
	}
	refString += baseImageURL.Path

	logger.Debug("parsing-reference", lager.Data{"refString": refString})
	transport := transports.Get(baseImageURL.Scheme)
	ref, err := transport.ParseReference(refString)
	if err != nil {
		return nil, errorspkg.Wrap(err, "parsing url failed")
	}

	return ref, nil
}
