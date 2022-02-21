package util

import "fmt"

var (
	extensions = []string{
		"7z",
		"aac",
		"ai",
		"apk",
		"ar",
		"avi",
		"bin",
		"bmp",
		"bz2",
		"cab",
		"cbr",
		"cbz",
		"crx",
		"deb",
		"dmg",
		"doc",
		"docx",
		"dwg",
		"dxf",
		"ebook",
		"egg",
		"eot",
		"eps",
		"epub",
		"exe",
		"flac",
		"flv",
		"gif",
		"gpx",
		"gz",
		"iso",
		"jpeg",
		"jpg",
		"kml",
		"kmz",
		"m4a",
		"mkv",
		"mobi",
		"mov",
		"mp3",
		"mp4",
		"mpeg",
		"mpg",
		"msg",
		"msi",
		"odp",
		"ods",
		"ogg",
		"ogm",
		"otf",
		"pak",
		"pdf",
		"pickle",
		"pkl",
		"png",
		"ppt",
		"ps",
		"psd",
		"rar",
		"rpm",
		"rst",
		"rtf",
		"s7z",
		"shar",
		"sketch",
		"svg",
		"tar",
		"tbz2",
		"tgz",
		"tif",
		"tiff",
		"tlz",
		"ttf",
		"war",
		"wav",
		"webp",
		"whl",
		"wma",
		"wmv",
		"woff",
		"woff2",
		"xls",
		"xlsx",
		"xpi",
		"zip",
		"zipx",
	}

	noiseDirectories = []string{
		".git",
		".idea",
		"node_modules",
		"bin",
		"debug",
		"release",
		"build",
		"obj",
		"target",
		"venv",
		"dist",
		"app_data",
		"lib",
		"lib64",
		"__pycache__",
		".cache",
	}

	extensionsMap map[string]bool
)

func init() {
	extensionsMap = make(map[string]bool, len(extensions))
	for _, ext := range extensions {
		extensionsMap[ext] = true
	}
}

func NotTextExt(ext string) bool {
	if len(ext) == 0 {
		// we don't assume anything if there's no extension
		return false
	}
	ext = ext[1:]
	return extensionsMap[ext]
}

func NoisyDirectoryExclusionPatterns() []string {
	patterns := make([]string, len(noiseDirectories))
	for i, dirname := range noiseDirectories {
		patterns[i] = fmt.Sprintf("**/%v/**", dirname)
	}
	return patterns
}
