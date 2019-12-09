package main

import (
	"bufio"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
)

func sameCasing(perms []string) bool {
	// Filter out numbers
	var alpha []string
	for _, word := range perms {
		if _, err := strconv.Atoi(word); err == nil {
			continue
		}
		alpha = append(alpha, word)
	}

	// Give an value for upper or lower
	style := make([]int, len(alpha))
	for index, word := range alpha {
		if strings.ToLower(word) != word {
			style[index] = 1
		} else {
			style[index] = -1
		}
	}

	// Check if the values match
	isConsistent := true
	for i := 1; i < len(style); i++ {
		if style[i-1] != style[i] {
			isConsistent = false
			break
		}
	}
	return isConsistent
}

func Permutations(sets []WordSet) (retVal [][]string) {
	var words []string
	for _, set := range sets {
		for _, word := range set.Words {
			found := false
			for _, w := range words {
				if word == w {
					found = true
					break
				}
			}
			if !found {
				words = append(words, word)
			}
		}
	}
	log.Println(len(words), words)

	// Find required word index
	required := -1
	for index, word := range words {
		if word == "81611" {
			required = index
			break
		}
	}

	if required == -1 {
		log.Fatalln("Missing required item")
	}

	// Generate 5 item
	for a := 0; a < len(words); a++ {
		for b := 0; b < len(words); b++ {
			// Ignore duplicates
			if b == a {
				continue
			}
			for c := 0; c < len(words); c++ {
				// Ignore duplicates
				if c == b || c == a {
					continue
				}
				for d := 0; d < len(words); d++ {
					// Ignore duplicates
					if d == c || d == b || d == a {
						continue
					}
					for e := 0; e < len(words); e++ {
						// Ignore duplicates
						if e == d || e == c || e == b || e == a {
							continue
						}

						// Make sure we have the required item
						if a == required || b == required || c == required || d == required || e == required {
							perm := []string{words[a], words[b], words[c], words[d], words[e]}

							// Make sure case is consistent
							if !sameCasing(perm) {
								continue
							}

							// Filter out any that contain duplicates from a set
							duplicateSetItems := false
							for _, set := range sets {
								if len(set.Words) < 2 {
									continue
								}

								equal := 0
								for _, word := range set.Words {
									for _, item := range perm {
										if word == item {
											equal++
										}
									}
								}

								if equal > 1 {
									duplicateSetItems = true
								}
							}

							if !duplicateSetItems {
								retVal = append(retVal, perm)
							}
						}
					}
				}
			}
		}
	}
	return
}

type WordSet struct {
	Words []string
}

func ToWordSet(w []string) WordSet {
	return WordSet{Words: w}
}

func main() {
	providers := []string{"bit.ly", "bit.do"} //"tiny.cc", "is.gd", "kutt.it"

	items := []WordSet{
		ToWordSet([]string{"US", "us", "USA", "usa"}),
		ToWordSet([]string{"Colorado", "colorado", "CO", "co", "83"}),
		ToWordSet([]string{"Jen", "jen"}),
		ToWordSet([]string{"Pitkin", "970", "pitkin"}),
		ToWordSet([]string{"Aspen", "aspen"}),
		ToWordSet([]string{"81611"}),
		ToWordSet([]string{"Maroon", "maroon"}),
		ToWordSet([]string{"Maroon", "maroon", "MaroonLake", "maroonlake"}),
		ToWordSet([]string{"Lake", "lake", "MaroonLake", "maroonlake"}),
		ToWordSet([]string{"Maroon", "maroon", "MaroonBells", "maroonbells"}),
		ToWordSet([]string{"Bells", "bells", "MaroonBells", "maroonbells"}),
		ToWordSet([]string{"Snowmass", "snowmass", "SnowmassVillage", "snowmassvillage"}),
	}

	results := Permutations(items)
	log.Println(len(results))

	fOut, err := os.Create("permutations.txt")
	if err != nil {
		log.Fatalln(err)
	}
	defer fOut.Close()

	for _, provider := range providers {
		w := bufio.NewWriter(fOut)
		for _, perm := range results {
			query := strings.Join(perm, "")

			if provider == "tinyurl.com" {
				if len(query) > 30 {
					continue
				}
				query = strings.ToLower(query)
			}

			u := url.URL{Scheme: "https", Host: provider, Path: query}
			fmt.Fprintln(w, u.String())
		}
		w.Flush()
	}
}
