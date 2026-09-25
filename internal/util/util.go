package util

import (
	"math/rand/v2"
)

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
