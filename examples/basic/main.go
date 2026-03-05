package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/QA-DNA/gowright"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("Launching browser...")
	b, err := gowright.Launch(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer b.Close()

	version, err := b.Version()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Browser: %s\n", version)

	fmt.Println("Creating context...")
	bc, err := b.NewContext()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Creating page...")
	page, err := bc.NewPage()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Navigating to example.com...")
	if err := page.Goto("https://example.com"); err != nil {
		log.Fatal(err)
	}

	title, err := page.Title()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Page title: %s\n", title)

	fmt.Println("Taking screenshot...")
	data, err := page.Screenshot()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Screenshot: %d bytes\n", len(data))

	fmt.Println("Closing...")
	bc.Close()
	fmt.Println("Done!")
}
