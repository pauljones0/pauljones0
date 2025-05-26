package main

import (
	"encoding/json" // Added for LD+JSON parsing
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/net/html"
)

// GetComicImage fetches a comic image or its URL from gocomics.com.
// comicName: The name of the comic strip (e.g., "garfield", "calvinandhobbes").
// year, month, day: The date of the comic. Month and day are 1-indexed.
// urlOnly: If true, returns the image URL as a string. Otherwise, returns image data as []byte.
func GetComicImage(comicName string, year int, month int, day int, urlOnly bool) (interface{}, error) {
	// 1. URL Construction
	// Ensure month and day are zero-padded in the URL (e.g., 2024/01/01).
	dateStr := fmt.Sprintf("%d/%02d/%02d", year, month, day)
	comicURL := fmt.Sprintf("https://www.gocomics.com/%s/%s", comicName, dateStr)

	fmt.Printf("Fetching HTML from: %s\n", comicURL)

	// 2. HTML Fetching
	resp, err := http.Get(comicURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch HTML from %s: %w", comicURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read body for more details if possible, even on error
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch HTML: status code %d for %s. Body: %s", resp.StatusCode, comicURL, string(bodyBytes))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read HTML body from %s: %w", comicURL, err)
	}

	// 3. HTML Parsing
	doc, err := html.Parse(strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML from %s: %w", comicURL, err)
	}

	imageSrcURL, err := extractImageURLFromNode(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to extract image URL from HTML of %s: %w", comicURL, err)
	}

	fmt.Printf("Extracted image URL: %s\n", imageSrcURL)

	// 4. Return Value
	if urlOnly {
		return imageSrcURL, nil
	}

	// Fetch the image data
	fmt.Printf("Fetching image data from: %s\n", imageSrcURL)
	imgResp, err := http.Get(imageSrcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image from %s: %w", imageSrcURL, err)
	}
	defer imgResp.Body.Close()

	if imgResp.StatusCode != http.StatusOK {
		// Read body for more details if possible
		imgBodyBytes, _ := io.ReadAll(imgResp.Body)
		return nil, fmt.Errorf("failed to fetch image: status code %d for %s. Body: %s", imgResp.StatusCode, imageSrcURL, string(imgBodyBytes))
	}

	imgData, err := io.ReadAll(imgResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data from %s: %w", imageSrcURL, err)
	}

	return imgData, nil
}

// parseLDJSONScriptContent attempts to extract an image URL from a single LD+JSON script's content.
// It prioritizes ImageObjects marked as representativeOfPage.
// Returns: the best URL found, a boolean indicating if it was from a representativeOfPage object, and an error.
func parseLDJSONScriptContent(jsonData string) (string, bool, error) {
	// Define LDJSONImageObject struct locally for parsing relevant fields
	type ldJSONImageObject struct {
		Type                 string `json:"@type"`
		URL                  string `json:"url"`
		ContentURL           string `json:"contentUrl"`
		RepresentativeOfPage bool   `json:"representativeOfPage"`
	}

	var collectedImageObjects []ldJSONImageObject

	// Helper to process a potential JSON object (map) and add it to collectedImageObjects if it's a valid ImageObject
	addImageObjectFromMap := func(itemMap map[string]interface{}) {
		if t, ok := itemMap["@type"].(string); ok && t == "ImageObject" {
			obj := ldJSONImageObject{Type: t}
			if u, okU := itemMap["url"].(string); okU {
				obj.URL = u
			}
			if cu, okCU := itemMap["contentUrl"].(string); okCU {
				obj.ContentURL = cu
			}
			if rep, okRep := itemMap["representativeOfPage"].(bool); okRep {
				obj.RepresentativeOfPage = rep
			}

			// Only add if there's a URL
			if obj.URL != "" || obj.ContentURL != "" {
				collectedImageObjects = append(collectedImageObjects, obj)
			}
		}
	}

	// Attempt to unmarshal jsonData in various common LD+JSON structures

	// 1. Try as a single JSON object
	var singleObjMap map[string]interface{}
	if err := json.Unmarshal([]byte(jsonData), &singleObjMap); err == nil {
		addImageObjectFromMap(singleObjMap)
	}

	// 2. Try as an array of JSON objects
	//    This handles cases where the root is an array of items.
	//    If a single ImageObject was already found, this step might be redundant unless the array contains a better one.
	//    For simplicity, we process it and let the prioritization logic sort it out.
	//    However, to avoid double-adding if the single object was part of an array that also got parsed,
	//    we only proceed if the single object parsing didn't yield an ImageObject or if it's a different structure.
	if len(collectedImageObjects) == 0 || (len(collectedImageObjects) > 0 && collectedImageObjects[0].Type != "ImageObject") {
		var arrObjMaps []map[string]interface{}
		if err := json.Unmarshal([]byte(jsonData), &arrObjMaps); err == nil {
			// If singleObjMap was parsed but wasn't an ImageObject, clear collectedImageObjects before processing array
			if len(collectedImageObjects) > 0 && collectedImageObjects[0].Type != "ImageObject" {
				collectedImageObjects = nil
			}
			for _, itemMap := range arrObjMaps {
				addImageObjectFromMap(itemMap)
			}
		}
	}

	// 3. Try as a root object containing a "@graph" array (if no ImageObjects found yet from direct parsing)
	//    This handles cases where ImageObjects are nested under a "@graph" key.
	if len(collectedImageObjects) == 0 {
		var rootGraphMap map[string]interface{}
		if err := json.Unmarshal([]byte(jsonData), &rootGraphMap); err == nil {
			if graph, ok := rootGraphMap["@graph"].([]interface{}); ok {
				for _, graphItemRaw := range graph {
					if graphItemMap, okGraphItem := graphItemRaw.(map[string]interface{}); okGraphItem {
						addImageObjectFromMap(graphItemMap)
					}
				}
			}
		}
	}

	if len(collectedImageObjects) == 0 {
		return "", false, errors.New("no ImageObject found in LD+JSON content")
	}

	// Select the best URL from collectedImageObjects:
	// - First priority: an ImageObject with representativeOfPage == true
	// - Second priority: the first ImageObject found (if no representative ones)
	var firstNonRepresentativeURL string
	for _, obj := range collectedImageObjects {
		currentURL := obj.URL
		if currentURL == "" {
			currentURL = obj.ContentURL
		}
		if currentURL == "" {
			continue // Skip if this object has no usable URL
		}

		if obj.RepresentativeOfPage {
			return currentURL, true, nil // Found representative, return immediately
		}
		if firstNonRepresentativeURL == "" {
			firstNonRepresentativeURL = currentURL // Capture the first non-representative URL
		}
	}

	if firstNonRepresentativeURL != "" {
		return firstNonRepresentativeURL, false, nil // Return first non-representative if no representative ones found
	}

	return "", false, errors.New("no ImageObject with a valid URL found in LD+JSON content")
}

// extractImageURLFromNode searches the HTML document for image URLs.
// Priority:
// 1. 'og:image' meta tag.
// 2. 'application/ld+json' script tag containing an ImageObject (preferring representativeOfPage:true).
// 3. 'twitter:image' meta tag.
func extractImageURLFromNode(doc *html.Node) (string, error) {
	var collectedOgImageURL string
	var collectedTwitterImageURL string

	// 1. Search for 'og:image' and 'twitter:image' meta tags
	var findMetaTagsRecursively func(*html.Node)
	findMetaTagsRecursively = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "meta" {
			var property, name, content string
			for _, attr := range n.Attr {
				if attr.Key == "property" {
					property = attr.Val
				}
				if attr.Key == "name" {
					name = attr.Val
				}
				if attr.Key == "content" {
					content = attr.Val
				}
			}

			if property == "og:image" && content != "" && collectedOgImageURL == "" {
				collectedOgImageURL = content // Capture first 'og:image'
			}
			if name == "twitter:image" && content != "" && collectedTwitterImageURL == "" {
				collectedTwitterImageURL = content // Capture first 'twitter:image'
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			// Optimization: if 'og:image' (primary target) is found, can stop searching for meta tags.
			// However, the recursive structure makes early exit from all branches tricky without a shared flag.
			// The check after the initial call to findMetaTagsRecursively handles this.
			findMetaTagsRecursively(c)
		}
	}
	findMetaTagsRecursively(doc)

	if collectedOgImageURL != "" {
		return collectedOgImageURL, nil // Priority 1: og:image
	}

	// 2. Search for 'application/ld+json' script tags if 'og:image' was not found
	var ldJSONURL string
	var foundRepresentativeLDJSON bool // Flag to stop search once a representative image is found

	var findLDJSONScriptsRecursively func(*html.Node)
	findLDJSONScriptsRecursively = func(n *html.Node) {
		if foundRepresentativeLDJSON { // If a representative image URL is already found, stop searching
			return
		}

		if n.Type == html.ElementNode && n.Data == "script" {
			isLDJSON := false
			for _, attr := range n.Attr {
				if attr.Key == "type" && attr.Val == "application/ld+json" {
					isLDJSON = true
					break
				}
			}
			if isLDJSON && n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
				jsonData := n.FirstChild.Data
				urlFromScript, wasRepresentative, err := parseLDJSONScriptContent(jsonData)
				if err == nil && urlFromScript != "" {
					if wasRepresentative {
						ldJSONURL = urlFromScript
						foundRepresentativeLDJSON = true // Set flag to stop further script searching
						return                           // Stop this branch of recursion
					} else if ldJSONURL == "" { // Only store the first non-representative URL if no representative one is found yet
						ldJSONURL = urlFromScript
					}
				}
			}
		}

		for c := n.FirstChild; c != nil && !foundRepresentativeLDJSON; c = c.NextSibling {
			findLDJSONScriptsRecursively(c)
			if foundRepresentativeLDJSON { // Propagate stop signal if a deeper call found a representative image
				return
			}
		}
	}
	findLDJSONScriptsRecursively(doc)

	if ldJSONURL != "" { // This will be set if a representative was found, or the first non-representative
		return ldJSONURL, nil // Priority 2: LD+JSON
	}

	// 3. Fallback to 'twitter:image' if 'og:image' and LD+JSON script failed
	if collectedTwitterImageURL != "" {
		return collectedTwitterImageURL, nil // Priority 3: twitter:image
	}

	return "", errors.New("HTML parsing error: could not find 'og:image' meta tag, suitable 'application/ld+json' script with an ImageObject, or 'twitter:image' meta tag")
}

func main() {
	comicName := "calvinandhobbes"
	year := 2024
	month := 1
	day := 1

	fmt.Printf("--- Test Case 1: Get Image URL Only ---\n")
	imgURLInterface, err := GetComicImage(comicName, year, month, day, true)
	if err != nil {
		fmt.Printf("Error getting image URL: %v\n", err)
	} else {
		imgURL, ok := imgURLInterface.(string)
		if !ok {
			fmt.Printf("Error: Expected string for image URL, got %T\n", imgURLInterface)
		} else {
			fmt.Printf("Successfully retrieved Image URL: %s\n", imgURL)
		}
	}

	fmt.Printf("\n--- Test Case 2: Get Image Data ---\n")
	imgDataInterface, err := GetComicImage(comicName, year, month, day, false)
	if err != nil {
		fmt.Printf("Error getting image data: %v\n", err)
	} else {
		imgData, ok := imgDataInterface.([]byte)
		if !ok {
			fmt.Printf("Error: Expected []byte for image data, got %T\n", imgDataInterface)
		} else {
			fmt.Printf("Successfully retrieved image data (size: %d bytes).\n", len(imgData))
			// For verification, you could save this data to a file, e.g.:
			// import "os"
			// os.WriteFile("calvinandhobbes_2024_01_01.jpg", imgData, 0644) // Ensure correct extension
		}
	}
}
