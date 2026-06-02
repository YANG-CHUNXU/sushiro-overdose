package main

import "github.com/Ryujoxys/sushiro-overdose/internal/app"

// Version is set via ldflags at build time (-X main.Version=...) and passed
// through to the app package. Kept in package main so existing build configs work.
var Version = "dev"

func main() {
	app.SetVersion(Version)
	app.Run()
}
