package spec

type DriverSpec struct {
	Type           string `json:"type"`
	StorePath      string `json:"store_path"`
	FsBinaryPath   string `json:"fs_binary_path"`
	MkfsBinaryPath string `json:"mkfs_binary_path"`
	SuidBinaryPath string `json:"suid_binary_path"`
}
