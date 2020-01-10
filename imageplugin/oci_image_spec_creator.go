package imageplugin

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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
	DiffID    string
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

	imageConfig := o.ImageConfigGenerator(baseLayer.DiffID, topLayer.DiffID)
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

func InsertDirsIntoTar(r *tar.Reader, w *tar.Writer, prefix string) error {
	for {
		hdr, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// if hdr.Name == "./" {
		// 	hdr.Mode = 0755
		// }
		hdr.Name = filepath.Join(prefix, hdr.Name)
		err = w.WriteHeader(hdr)
		if err != nil {
			return err
		}

		fileContents, err := ioutil.ReadAll(r)
		if err != nil {
			return err
		}

		_, err = w.Write(fileContents)
		if err != nil {
			return err
		}
	}
	return nil
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

	resp, err := client.Get(topLayerURL)
	if err != nil {
		return errs(err)
	}

	gunzippedInput, err := gzip.NewReader(resp.Body)
	if err != nil {
		fmt.Printf("err = %+v\n", err)
		return errs(err)
	}

	tarOutputBuf := bytes.NewBuffer([]byte{})

	tarReader := tar.NewReader(gunzippedInput)
	tarWriter := tar.NewWriter(tarOutputBuf)
	err = InsertDirsIntoTar(tarReader, tarWriter, "/home/vcap")
	if err != nil {
		fmt.Printf("err = %+v\n", err)
		return errs(err)
	}
	fmt.Printf("transformed tar bytes length %d\n", len(tarOutputBuf.Bytes()))

	err = tarWriter.Close()
	if err != nil {
		fmt.Printf("err = %+v\n", err)
		return errs(err)
	}

	tarOutputBytes := tarOutputBuf.Bytes()

	tgzOutputBuf := bytes.NewBuffer([]byte{})
	gzipWriter := gzip.NewWriter(tgzOutputBuf)

	_, err = io.Copy(gzipWriter, tarOutputBuf)
	if err != nil {
		fmt.Printf("err = %+v\n", err)
		return errs(err)
	}

	err = gzipWriter.Close()
	if err != nil {
		fmt.Printf("err = %+v\n", err)
		return errs(err)
	}

	defer gunzippedInput.Close()
	defer resp.Body.Close()

	topLayerDiffID := digest.FromBytes(tarOutputBytes)
	// ioutil.WriteFile("/tmp/spec.layer.tar", tarOutputBytes, 0644)
	topLayerDigest := digest.FromBytes(tgzOutputBuf.Bytes())
	// ioutil.WriteFile("/tmp/spec.layer.tar.gz", tgzOutputBuf.Bytes(), 0644)
	topLayerSize := len(tgzOutputBuf.Bytes())
	fmt.Printf("topLayerSize = %+v\n", topLayerSize)

	topLayerPath, err := getQueryValue(rootFS, "layer_path")
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
		SHA256: rootfsPathSHA.Hex(),
		// Base layer is uncompressed anyway, so sha and diffIDs are the same
		DiffID:    rootfsPathSHA.Hex(),
		MediaType: "application/vnd.oci.image.layer.v1.tar",
		Size:      rootFSPathFile.Size(),
	}

	topLayer := Layer{
		URL:       topLayerURL,
		SHA256:    topLayerDigest.Hex(),
		DiffID:    topLayerDiffID.Hex(),
		BaseDir:   topLayerPath,
		MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
		Size:      int64(topLayerSize),
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
