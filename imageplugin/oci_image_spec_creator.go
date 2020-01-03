package imageplugin

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	digest "github.com/opencontainers/go-digest"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
)

type OCIImageSpecCreator struct {
	DepotDir             string
	ImageConfigGenerator func(layerSHAs ...string) imagespec.Image
	ManifestGenerator    func(layers []Layer, configSHA string, configSize int64) imagespec.Manifest
	IndexGenerator       func(manifestSHA string, manifestSize int64) imagespec.Index
}

func NewOCIImageSpecCreator(depotDir string) *OCIImageSpecCreator {
	return &OCIImageSpecCreator{
		DepotDir:             depotDir,
		ImageConfigGenerator: GenerateImageConfig,
		ManifestGenerator:    GenerateManifest,
		IndexGenerator:       GenerateIndex,
	}
}

type Layer struct {
	URL       string
	SHA256    string
	BaseDir   string
	MediaType string
	Size      int64
}

func (o *OCIImageSpecCreator) CreateImageSpec(rootFS *url.URL, handle string) (*url.URL, error) {
	imageDir := filepath.Join(o.DepotDir, handle, "image")
	blobsPath := filepath.Join(imageDir, "blobs", "sha256")
	if err := os.MkdirAll(blobsPath, 0755); err != nil {
		return nil, err
	}

	err := ioutil.WriteFile(filepath.Join(imageDir, imagespec.ImageLayoutFile), []byte(fmt.Sprintf(`{"imageLayoutVersion": "%s"}`, imagespec.ImageLayoutVersion)), 0600)
	if err != nil {
		return nil, err
	}

	baseLayer, topLayer, err := layers(rootFS)
	if err != nil {
		return nil, err
	}

	imageConfig := o.ImageConfigGenerator(baseLayer.SHA256, topLayer.SHA256)
	imageConfigBytes, err := json.Marshal(imageConfig)
	if err != nil {
		return nil, err
	}
	imageConfigSHA := shaOf(imageConfigBytes)
	if err = ioutil.WriteFile(filepath.Join(blobsPath, imageConfigSHA), imageConfigBytes, 0600); err != nil {
		return nil, err
	}

	if err = os.Symlink(rootFS.Path, filepath.Join(blobsPath, baseLayer.SHA256)); err != nil {
		return nil, err
	}

	manifest := o.ManifestGenerator([]Layer{baseLayer, topLayer}, imageConfigSHA, int64(len(imageConfigBytes)))
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	manifestSHA := shaOf(manifestBytes)
	if err = ioutil.WriteFile(filepath.Join(blobsPath, manifestSHA), manifestBytes, 0600); err != nil {
		return nil, err
	}

	index := o.IndexGenerator(manifestSHA, int64(len(manifestBytes)))
	indexBytes, err := json.Marshal(index)
	if err != nil {
		return nil, err
	}
	if err := ioutil.WriteFile(filepath.Join(imageDir, "index.json"), indexBytes, 0600); err != nil {
		return nil, err
	}

	return url.Parse(fmt.Sprintf("oci://%s", forwardSlashesOnly(imageDir)))
}

func layers(rootFS *url.URL) (Layer, Layer, error) {
	errs := func(err error) (Layer, Layer, error) {
		return Layer{}, Layer{}, err
	}

	if rootFS.Scheme != PreloadedPlusLayerScheme {
		return errs(fmt.Errorf("scheme '%s' not supported: expected %s", rootFS.Scheme, PreloadedPlusLayerScheme))
	}

	topLayerURL, err := getQueryValue(rootFS, "layer")
	if err != nil {
		return errs(err)
	}

	client, err := getHTTPClientWithCerts()
	if err != nil {
		return errs(err)
	}

	resp, err := client.Head(topLayerURL)
	if err != nil {
		return errs(err)
	}

	topLayerSize := resp.ContentLength

	topLayerPath, err := getQueryValue(rootFS, "layer_path")
	if err != nil {
		return errs(err)
	}

	topLayerDigest, err := getQueryValue(rootFS, "layer_digest")
	if err != nil {
		return errs(err)
	}

	rootFSPathFile, err := os.Stat(rootFS.Path)
	if err != nil {
		return errs(err)
	}
	// rootFSPathMtime := rootFSPathFile.ModTime().UnixNano()

	// This could be extreeemely slow...
	rootfsPathSHA, err := shaOfFile(rootFS.Path)
	if err != nil {
		return errs(err)
	}

	baseLayer := Layer{
		SHA256:    rootfsPathSHA.Hex(),
		MediaType: "application/vnd.oci.image.layer.v1.tar",
		Size:      rootFSPathFile.Size(),
	}

	topLayer := Layer{
		URL:       topLayerURL,
		SHA256:    topLayerDigest,
		BaseDir:   topLayerPath,
		MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
		Size:      topLayerSize,
	}

	return baseLayer, topLayer, nil
}

func shaOfFile(filePath string) (digest.Digest, error) {
	r, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer r.Close()
	return digest.FromReader(r)
}

func getHTTPClientWithCerts() (*http.Client, error) {
	certsDir := "/var/vcap/jobs/garden/certs"
	certFile := filepath.Join(certsDir, "remote-layer.cert")
	keyFile := filepath.Join(certsDir, "remote-layer.key")
	caFile := filepath.Join(certsDir, "remote-layer.crt")

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	caCert, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}
	tlsConfig.BuildNameToCertificate()
	transport := &http.Transport{TLSClientConfig: tlsConfig}
	return &http.Client{Transport: transport}, nil
}

func getQueryValue(u *url.URL, key string) (string, error) {
	value, ok := u.Query()[key]
	if !ok {
		return "", fmt.Errorf("no query parameter '%s' found in URL %s", key, u.String())
	}
	return value[0], nil
}

func shaOf(data []byte) string {
	sha := sha256.Sum256(data)
	return hex.EncodeToString(sha[:])
}

// On Windows, temp dir paths will contain backslashes.
// However, a valid Windows file URI uses forward slashes, e.g.
// file://C:/some/path
func forwardSlashesOnly(pathname string) string {
	return strings.Replace(pathname, `\`, "/", -1)
}
