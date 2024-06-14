package ui

import (
	"embed"
	"fmt"
	"math/rand"
	"time"
)

//go:embed banners/*.txt
var bannerDir embed.FS

func randomBanner() string {
	rand.Seed(time.Now().UnixNano())

	bannerFiles, _ := bannerDir.ReadDir("banners")
	file, _ := bannerDir.ReadFile("banners/" + bannerFiles[rand.Intn(len(bannerFiles))].Name())

	return fmt.Sprintf("\n\n\nWelcome to...\n\n[red::b]%s[-:-:-]\n\n", file)
}
