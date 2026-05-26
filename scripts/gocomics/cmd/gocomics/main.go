package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/pauljones0/gocomics"
)

func main() {
	comicNameFlag := flag.String("comic-name", "", "Name of the comic (e.g., \"calvinandhobbes\", required)")
	yearFlag := flag.Int("year", 0, "Year of the comic (optional, defaults to current year)")
	monthFlag := flag.Int("month", 0, "Month of the comic (optional, defaults to current month)")
	dayFlag := flag.Int("day", 0, "Day of the comic (optional, defaults to current day)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s --comic-name <name> [--year YYYY] [--month MM] [--day DD]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "This tool fetches and prints the comic image URL to stdout.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *comicNameFlag == "" {
		fmt.Fprintf(os.Stderr, "Error: --comic-name flag is required.\n\n")
		flag.Usage()
		os.Exit(1)
	}

	now := time.Now()
	year := *yearFlag
	if year == 0 {
		year = now.Year()
	}

	month := *monthFlag
	if month == 0 {
		month = int(now.Month())
	}

	day := *dayFlag
	if day == 0 {
		day = now.Day()
	}

	client := gocomics.NewClient()
	imgURL, err := client.GetComicImageURL(*comicNameFlag, year, month, day)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching comic image URL: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(imgURL)
}
