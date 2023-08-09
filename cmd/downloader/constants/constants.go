package constants

const (
	AppName                 = "catalyst"
	LatestTagReleaseName    = "latest"
	SignatureFileExtension  = "sig"
	ChecksumFileSuffix      = "checksums.txt"
	TaggedDownloadURLFormat = "https://github.com/%s/releases/download/%s/%s"
	BucketDownloadURLFormat = "https://build.livepeer.live/%s/%s/%s"
	BucketManifestURLFormat = "https://build.livepeer.live/%s/%s.json"
	PGPKeyFingerprint       = "A2F9039A8603C44C21414432A2224D4537874DB2"
	ZipFileExtension        = "zip"
	TarFileExtension        = "tar.gz"
)

const PGPPublicKey = `-----BEGIN PGP PUBLIC KEY BLOCK-----
Comment: A2F9 039A 8603 C44C 2141  4432 A222 4D45 3787 4DB2
Comment: Livepeer CI Robot <support@livepeer.org>

xjMEYfwEERYJKwYBBAHaRw8BAQdAIA1ob/8L+GwvWZLhgJYpPu4yzGm/GGIQnsyn
hiKGIZ/NKExpdmVwZWVyIENJIFJvYm90IDxzdXBwb3J0QGxpdmVwZWVyLm9yZz7C
lgQTFggAPhYhBKL5A5qGA8RMIUFEMqIiTUU3h02yBQJh/AQRAhsDBQkJZgGABQsJ
CAcCBhUKCQgLAgQWAgMBAh4BAheAAAoJEKIiTUU3h02yuW0A/RCyDnQiJrbquYpx
4SJEifUu5fls26lfzeDAfmGcrRV3AQCSiedYh3BKmNFRWY3DJcUJeLuQbjwq8K7e
amgxkyADAc44BGH8BBESCisGAQQBl1UBBQEBB0AOEGZ+On72yLknC3ARUmSWqOhd
YEFE2hNQKnh9R7VkKgMBCAfCfgQYFggAJhYhBKL5A5qGA8RMIUFEMqIiTUU3h02y
BQJh/AQRAhsMBQkJZgGAAAoJEKIiTUU3h02ypTgA/1QzMevOG9v0gYxCrTyFmCMD
d2Nyp2+Tl88mtlOceWi9AQCEwE58KLd/EAodO7tEE8igDdHwvrD+cJS/wrJSha5W
Cg==
=7UZ1
-----END PGP PUBLIC KEY BLOCK-----`
