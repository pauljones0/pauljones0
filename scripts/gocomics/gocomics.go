package gocomics

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/net/html"
)

// Client is a gocomics client that can fetch comic image URLs.
type Client struct {
	HTTPClient *http.Client
	BaseURL    string
}

// NewClient returns a new gocomics client with default settings.
func NewClient() *Client {
	return &Client{
		HTTPClient: &http.Client{Timeout: 15 * time.Second},
		BaseURL:    "https://www.gocomics.com",
	}
}

// GetComicImageURL fetches a comic image URL from gocomics.com.
func (c *Client) GetComicImageURL(comicName string, year int, month int, day int) (string, error) {
	dateStr := fmt.Sprintf("%d/%02d/%02d", year, month, day)
	comicURL := fmt.Sprintf("%s/%s/%s", c.BaseURL, comicName, dateStr)

	req, err := http.NewRequest("GET", comicURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request for %s: %w", comicURL, err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch HTML from %s: %w", comicURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to fetch HTML: status code %d for %s. Body: %s", resp.StatusCode, comicURL, string(bodyBytes))
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML from %s: %w", comicURL, err)
	}

	return extractImageURLFromNode(doc)
}

// GetComicImageURL is a convenience function that uses a default client.
func GetComicImageURL(comicName string, year int, month int, day int) (string, error) {
	return NewClient().GetComicImageURL(comicName, year, month, day)
}

func parseLDJSONScriptContent(jsonData string) (string, bool, error) {
	type ldJSONImageObject struct {
		Type                 string `json:"@type"`
		URL                  string `json:"url"`
		ContentURL           string `json:"contentUrl"`
		RepresentativeOfPage bool   `json:"representativeOfPage"`
	}

	var collectedImageObjects []ldJSONImageObject

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

			if obj.URL != "" || obj.ContentURL != "" {
				collectedImageObjects = append(collectedImageObjects, obj)
			}
		}
	}

	var singleObjMap map[string]interface{}
	if err := json.Unmarshal([]byte(jsonData), &singleObjMap); err == nil {
		addImageObjectFromMap(singleObjMap)
	}

	if len(collectedImageObjects) == 0 {
		var arrObjMaps []map[string]interface{}
		if err := json.Unmarshal([]byte(jsonData), &arrObjMaps); err == nil {
			for _, itemMap := range arrObjMaps {
				addImageObjectFromMap(itemMap)
			}
		}
	}

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

	var firstNonRepresentativeURL string
	for _, obj := range collectedImageObjects {
		currentURL := obj.URL
		if currentURL == "" {
			currentURL = obj.ContentURL
		}
		if currentURL == "" {
			continue
		}

		if obj.RepresentativeOfPage {
			return currentURL, true, nil
		}
		if firstNonRepresentativeURL == "" {
			firstNonRepresentativeURL = currentURL
		}
	}

	if firstNonRepresentativeURL != "" {
		return firstNonRepresentativeURL, false, nil
	}

	return "", false, errors.New("no ImageObject with a valid URL found in LD+JSON content")
}

func extractImageURLFromNode(doc *html.Node) (string, error) {
	var collectedOgImageURL string
	var collectedTwitterImageURL string

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
				collectedOgImageURL = content
			}
			if name == "twitter:image" && content != "" && collectedTwitterImageURL == "" {
				collectedTwitterImageURL = content
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findMetaTagsRecursively(c)
		}
	}
	findMetaTagsRecursively(doc)

	if collectedOgImageURL != "" {
		return collectedOgImageURL, nil
	}

	var ldJSONURL string
	var foundRepresentativeLDJSON bool

	var findLDJSONScriptsRecursively func(*html.Node)
	findLDJSONScriptsRecursively = func(n *html.Node) {
		if foundRepresentativeLDJSON {
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
						foundRepresentativeLDJSON = true
						return
					} else if ldJSONURL == "" {
						ldJSONURL = urlFromScript
					}
				}
			}
		}

		for c := n.FirstChild; c != nil && !foundRepresentativeLDJSON; c = c.NextSibling {
			findLDJSONScriptsRecursively(c)
			if foundRepresentativeLDJSON {
				return
			}
		}
	}
	findLDJSONScriptsRecursively(doc)

	if ldJSONURL != "" {
		return ldJSONURL, nil
	}

	if collectedTwitterImageURL != "" {
		return collectedTwitterImageURL, nil
	}

	return "", errors.New("could not find image URL in meta tags or LD+JSON")
}
