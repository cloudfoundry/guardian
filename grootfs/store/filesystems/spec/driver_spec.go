package spec

type DriverSpec struct {
	Type           string `json:"type"`
	StorePath      string `json:"store_path"`
	SuidBinaryPath string `json:"suid_binary_path"`
}
