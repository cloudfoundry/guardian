package nerdimage

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/guardian/imageplugin"
	"code.cloudfoundry.org/tlsconfig"
	"github.com/hashicorp/go-multierror"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type BlobstoreFetcher struct {
	ociImageDir string
}

func NewBlobstoreFetcher(ociImageDir string) *BlobstoreFetcher {
	return &BlobstoreFetcher{ociImageDir: ociImageDir}
}

func (f BlobstoreFetcher) Fetch(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error) {
	fmt.Printf("FETCH!!! %+v\n", desc)
	if desc.MediaType == ocispec.MediaTypeImageIndex {
		return os.Open(filepath.Join(f.ociImageDir, "index.json"))
	}

	if len(desc.URLs) == 0 {
		return os.Open(filepath.Join(f.ociImageDir, "blobs", desc.Digest.Algorithm().String(), desc.Digest.Encoded()))
	}

	client, err := createHTTPClient("/var/vcap/jobs/garden/certs")
	if err != nil {
		return nil, err
	}
	var respErr *multierror.Error
	for _, url := range desc.URLs {
		resp, err := client.Get(url)
		if err != nil {
			respErr = multierror.Append(respErr, err)
			continue
		}

		if resp.StatusCode != 200 {
			respErr = multierror.Append(respErr, fmt.Errorf("unexpected response code: %d for url: %s", resp.StatusCode, url))
			resp.Body.Close()
			continue
		}

		defer resp.Body.Close()

		inputGzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			fmt.Printf("err = %+v\n", err)
			respErr = multierror.Append(respErr, err)
			continue
		}

		inputTarGzReader := tar.NewReader(inputGzReader)

		outputBuffer := bytes.NewBuffer([]byte{})
		outputTarWriter := tar.NewWriter(outputBuffer)

		err = imageplugin.InsertDirsIntoTar(inputTarGzReader, outputTarWriter, "/home/vcap")
		if err != nil {
			fmt.Printf("err = %+v\n", err)
			respErr = multierror.Append(respErr, err)
			continue
		}

		err = outputTarWriter.Close()
		if err != nil {
			fmt.Printf("err = %+v\n", err)
			respErr = multierror.Append(respErr, err)
			continue
		}
		ioutil.WriteFile("/tmp/fetcher.layer.tar", outputBuffer.Bytes(), 0644)

		outputGzBuffer := bytes.NewBuffer([]byte{})
		outputGzWriter := gzip.NewWriter(outputGzBuffer)
		_, err = io.Copy(outputGzWriter, outputBuffer)
		if err != nil {
			fmt.Printf("err = %+v\n", err)
			respErr = multierror.Append(respErr, err)
			continue
		}

		err = outputGzWriter.Close()
		if err != nil {
			fmt.Printf("err = %+v\n", err)
			respErr = multierror.Append(respErr, err)
			continue
		}
		ioutil.WriteFile("/tmp/fetcher.layer.tar.gz", outputGzBuffer.Bytes(), 0644)

		return ioutil.NopCloser(outputGzBuffer), nil
	}

	return nil, respErr
}

func createHTTPClient(certPath string) (*http.Client, error) {
	cacert := filepath.Join(certPath, "remote-layer.crt")
	cert := filepath.Join(certPath, "remote-layer.cert")
	key := filepath.Join(certPath, "remote-layer.key")

	return createTLSHTTPClient([]CertPaths{
		{Crt: cert, Key: key, Ca: cacert},
	})
}

type CertPaths struct {
	Crt, Key, Ca string
}

func createTLSHTTPClient(certPaths []CertPaths) (*http.Client, error) {
	tlsOpts := []tlsconfig.TLSOption{tlsconfig.WithInternalServiceDefaults()}
	tlsClientOpts := []tlsconfig.ClientOption{}

	for _, certPath := range certPaths {
		tlsOpts = append(tlsOpts, tlsconfig.WithIdentityFromFile(certPath.Crt, certPath.Key))
		tlsClientOpts = append(tlsClientOpts, tlsconfig.WithAuthorityFromFile(certPath.Ca))
	}

	tlsConfig, err := tlsconfig.Build(tlsOpts...).Client(tlsClientOpts...)
	if err != nil {
		return nil, err
	}

	return &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig}}, nil
}
