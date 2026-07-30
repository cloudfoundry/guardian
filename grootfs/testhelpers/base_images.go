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

var EmptyBaseImageV011 = BaseImage{
	ConfigBlobID: "sha256:217f3b4afdf698d639f854d9c6d640903a011413bc7e7bffeabe63c7ca7e4a7d",
	Layers: []Layer{
		{
			BlobID:  "sha256:47e3dd80d678c83c50cb133f4cf20e94d088f890679716c8b763418f55827a58",
			DiffID:  "sha256:afe200c63655576eaa5cabe036a2c09920d6aee67653ae75a9d35e0ec27205a5",
			ChainID: "afe200c63655576eaa5cabe036a2c09920d6aee67653ae75a9d35e0ec27205a5",
		},
		{
			BlobID:  "sha256:7f2760e7451ce455121932b178501d60e651f000c3ab3bc12ae5d1f57614cc76",
			DiffID:  "sha256:d7c6a5f0d9a15779521094fa5eaf026b719984fb4bfe8e0012bd1da1b62615b0",
			ChainID: "9242945d3c9c7cf5f127f9352fea38b1d3efe62ee76e25f70a3e6db63a14c233",
		},
	},
}

var SchemaV1EmptyBaseImage = BaseImage{
	ConfigBlobID: "sha256:673ffd41d87662c55979a69fa65390552c5d9ace57ad81fb8edb732c3e93e483",
	Layers: []Layer{
		{
			BlobID:  "sha256:52654756942eb47cd5e803d9d5905f2f5e8045f3794eed727dee5734ff485771",
			DiffID:  "sha256:5b9746c87396a8cff11e5cbfa37321018bb797fcf0b4404615e713337ea1c62b",
			ChainID: "5b9746c87396a8cff11e5cbfa37321018bb797fcf0b4404615e713337ea1c62b",
		},
		{
			BlobID:  "sha256:64df864e75bb2dea96b726cff208ce7ee8125901dfc11e53479534519735c8e4",
			DiffID:  "sha256:eb639428f6fd1693228c64a7474bd6302c7ebd06404c7092dd3d5187ba4fdddf",
			ChainID: "5726c173a767b58badc677d61d0e45df09f0d923f9770210f9a88607e1386e2b",
		},
		{
			BlobID:  "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4",
			DiffID:  "sha256:5f70bf18a086007016e948b04aed3b82103a36bea41755b6cddfaf10ace3c6ef",
			ChainID: "033ceceec9c0d8d02902f2cbca179bab1a47620874d0bb132d80af0307b0c671",
		},
	},
}

var BusyBoxImage = BaseImage{
	ConfigBlobID: "sha256:1efc1d465fd6255396d0efd90a899f57aba2b7b294b02d5f55c6f5505ca9f3e5",
	Layers: []Layer{
		{
			ChainID: "3d16a9d9679dba04b90edeeca13dfaa986a268a7e0f4764250bacc2bed236b73",
		},
	},
}
