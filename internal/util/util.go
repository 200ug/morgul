package util

import (
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

const VersionCode = "v2.2.1 (2026-08-14)"

var Hostnames = []string{
	"atomic",
	"yugen",
	"kinzai",
	"mono",
	"kogarashi",
	"franzk",
	"ichwan",
	"ketman",
	"karakul",
	"jadacha",
	"arrakeen",
	"logaki",
	"furusato",
	"kanbina",
	"kimyou",
	"fuga",
	"miyabi",
	"seijaku",
	"sakura",
	"gekko",
	"shinryoku",
	"shigure",
	"akogare",
	"yuki",
	"kibo",
	"yasuragi",
	"tokimeki",
	"hikari",
	"yami",
	"maboroshi",
	"oracle",
	"landsraad",
	"enfeil",
	"mirabhasa",
	"kindjal",
	"fuzei",
	"omokage",
	"magokoro",
	"yawaraka",
	"nukumori",
	"akatsuki",
	"yanluowang",
	"shigawire",
	"burseg",
	"sardaukar",
	"tlulax",
	"kralizec",
}

func RandomHostname() string {
	return Hostnames[rand.IntN(len(Hostnames))]
}

func CopyDir(src, dst string, exclude []string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		for _, pattern := range exclude {
			matched, _ := filepath.Match(pattern, rel)
			if matched {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return CopyFile(path, target)
	})
}

func CopyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0755)
}

func StripAllWhitespace(str string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, str)
}
