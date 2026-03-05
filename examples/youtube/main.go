package main

import (
	"context"
	"fmt"
	"time"

	"github.com/PeterStoica/gowright"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Launch reads from gowright.config.json automatically
	b, err := gowright.Launch(ctx)
	if err != nil {
		panic(err)
	}
	defer b.Close()

	bc, err := b.NewContext()
	if err != nil {
		panic(err)
	}

	page, err := bc.NewPage()
	if err != nil {
		panic(err)
	}

	fmt.Println("Navigating to YouTube...")
	if err := page.Goto("https://www.youtube.com"); err != nil {
		panic(err)
	}

	title, _ := page.Title()
	fmt.Printf("Page title: %s\n", title)

	fmt.Println("Clicking first video...")
	err = page.Locator("a#thumbnail").Click()
	if err != nil {
		panic(err)
	}

	time.Sleep(2 * time.Second)

	title, _ = page.Title()
	fmt.Printf("Video title: %s\n", title)

	fmt.Println("Watching for 5 seconds...")
	time.Sleep(5 * time.Second)

	fmt.Println("Done!")
}
