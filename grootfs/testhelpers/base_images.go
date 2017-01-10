package testhelpers

type Layer struct {
	BlobID  string
	DiffID  string
	ChainID string
}

type BaseImage struct {
	ConfigBlobID string
	Layers       []Layer
}

var EmptyBaseImageV01 = BaseImage{
	ConfigBlobID: "sha256:798d0b171b05f1cb0cf6f52d11cd39722182f2272b66488873bcfb8350917d2b",
	Layers: []Layer{
		{
			BlobID:  "sha256:6c1f4533b125f8f825188c4f4ff633a338cfce0db2813124d3d518028baf7d7a",
			DiffID:  "sha256:3355e23c079e9b35e4b48075147a7e7e1850b99e089af9a63eed3de235af98ca",
			ChainID: "sha256:3355e23c079e9b35e4b48075147a7e7e1850b99e089af9a63eed3de235af98ca",
		},
	},
}

var EmptyBaseImageV011 = BaseImage{
	ConfigBlobID: "sha256:217f3b4afdf698d639f854d9c6d640903a011413bc7e7bffeabe63c7ca7e4a7d",
	Layers: []Layer{
		{
			BlobID:  "sha256:47e3dd80d678c83c50cb133f4cf20e94d088f890679716c8b763418f55827a58",
			DiffID:  "sha256:afe200c63655576eaa5cabe036a2c09920d6aee67653ae75a9d35e0ec27205a5",
			ChainID: "sha256:afe200c63655576eaa5cabe036a2c09920d6aee67653ae75a9d35e0ec27205a5",
		},
		{
			BlobID:  "sha256:7f2760e7451ce455121932b178501d60e651f000c3ab3bc12ae5d1f57614cc76",
			DiffID:  "sha256:d7c6a5f0d9a15779521094fa5eaf026b719984fb4bfe8e0012bd1da1b62615b0",
			ChainID: "sha256:9242945d3c9c7cf5f127f9352fea38b1d3efe62ee76e25f70a3e6db63a14c233",
		},
	},
}

var BusyBoxImage = BaseImage{
	ConfigBlobID: "sha256:1efc1d465fd6255396d0efd90a899f57aba2b7b294b02d5f55c6f5505ca9f3e5",
	Layers: []Layer{
		{
			BlobID:  "sha256:fdab12439263ca89c449f09a489b6fbf40af8526a3dda279c8954451703641c3",
			DiffID:  "sha256:68737f5031b303067faacb264cd3b1466973da762bd83d4381e5396e6a4b79a8",
			ChainID: "sha256:c85de82f7789ba8696cf905ea603deccc1b082db21181117e1f74d1dd77adf47",
		},
	},
}
