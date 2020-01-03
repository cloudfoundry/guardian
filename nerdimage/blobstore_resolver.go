package nerdimage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/remotes"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type BlobstoreResolver struct {
	ociImageDir string
	handle      string
}

func NewBlobstoreResolver(ociImageDir string, handle string) *BlobstoreResolver {
	return &BlobstoreResolver{ociImageDir: ociImageDir, handle: handle}
}

func (r BlobstoreResolver) Resolve(ctx context.Context, ref string) (name string, desc ocispec.Descriptor, err error) {
	if ref == r.handle {
		indexPath := filepath.Join(r.ociImageDir, "index.json")
		indexSha, err := shaOf(indexPath)
		if err != nil {
			return "", ocispec.Descriptor{}, err
		}
		indexSize, err := sizeOf(indexPath)
		if err != nil {
			return "", ocispec.Descriptor{}, err
		}

		return r.handle,
			ocispec.Descriptor{
				MediaType: ocispec.MediaTypeImageIndex,
				Digest:    indexSha,
				Size:      indexSize,
				Platform:  &ocispec.Platform{Architecture: "amd64", OS: "linux"},
			},
			nil
	}

	return "", ocispec.Descriptor{}, fmt.Errorf("no resolver for ref %q", ref)
}

func (r BlobstoreResolver) Fetcher(ctx context.Context, ref string) (remotes.Fetcher, error) {
	return NewBlobstoreFetcher(r.ociImageDir), nil
}

func (r BlobstoreResolver) Pusher(ctx context.Context, ref string) (remotes.Pusher, error) {
	return nil, errors.New("pusher is not implmenented")
}

func shaOf(filePath string) (digest.Digest, error) {
	r, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer r.Close()
	return digest.FromReader(r)
}

func sizeOf(filePath string) (int64, error) {
	stat, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}

	size := stat.Size()
	return size, nil
}
