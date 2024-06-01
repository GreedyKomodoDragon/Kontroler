package utils

import (
	"math/rand"
)

// TODO: make this loadable from a configmap
var wordList = []string{
	"apple", "banana", "cherry", "orange", "grape", "pear",
	"dog", "cat", "rabbit", "hamster", "turtle", "goldfish",
	"carrot", "broccoli", "potato", "tomato", "onion", "pepper",
	"chair", "table", "lamp", "sofa", "desk", "bookcase",
}

func GenerateRandomName() string {
	// Initialize an empty name
	var name string

	// Choose a random number of words to combine (between 2 and 3)
	numWords := 3

	// Choose random words and combine them
	for i := 0; i < numWords; i++ {
		// Randomly select a word from the list
		wordIndex := rand.Intn(len(wordList))
		name += wordList[wordIndex]

		// If it's not the last word, add a space
		if i < numWords-1 {
			name += "-"
		}
	}

	return name
}
