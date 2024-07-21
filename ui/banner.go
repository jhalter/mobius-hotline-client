package ui

import (
	"embed"
	"fmt"
	"math/rand"
)

//go:embed banners/*.txt
var bannerDir embed.FS

func randomBanner() string {
	bannerFiles, _ := bannerDir.ReadDir("banners")
	file, _ := bannerDir.ReadFile("banners/" + bannerFiles[rand.Intn(len(bannerFiles))].Name())

	return fmt.Sprintf("\n\n\nWelcome to...\n\n[red::b]%s[-:-:-]\n\n", file)
}
