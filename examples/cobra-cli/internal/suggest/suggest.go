package suggest

import "strings"

// Breed represents a dog breed with a reason for the recommendation.
type Breed struct {
	Name   string
	Reason string
	Image  string
}

var breeds = []Breed{
	{"Shiba Inu", "mysterious, unknowable, and deeply unimpressed by everyone", "shiba_inu.jpg"},
	{"Dachshund", "short legs, big opinions, just like your emails", "dachshund.jpg"},
	{"Border Collie", "you have 47 tabs open right now, don't you", "border_collie.jpg"},
	{"Golden Retriever", "everyone already likes you, might as well lean in", "golden_retriever.jpg"},
	{"Pug", "chaotic energy, loveable, slightly concerning breathing", "pug.jpg"},
	{"Greyhound", "fast to start, impossible to stop once going", "greyhound.jpg"},
	{"Basset Hound", "wise, mournful, and deeply committed to doing nothing", "basset_hound.jpg"},
	{"Corgi", "secretly running everything, nobody knows how", "corgi.jpg"},
}

// CountVowels returns the number of vowels in the given name.
func CountVowels(name string) int {
	count := 0
	for _, c := range strings.ToLower(name) {
		switch c {
		case 'a', 'e', 'i', 'o', 'u':
			count++
		}
	}
	return count
}

// ForName returns a Breed recommendation based on the vowel count of the name.
func ForName(name string) Breed {
	return breeds[CountVowels(name)%len(breeds)]
}
