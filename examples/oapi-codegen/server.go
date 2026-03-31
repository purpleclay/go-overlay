package main

import (
	"context"
	"strings"

	"github.com/purpleclay/go-overlay/examples/oapi-codegen/gen"
)

var breedImages = []string{
	"sphinx.jpg",
	"scottish_fold.jpg",
	"maine_coon.jpg",
	"siamese.jpg",
	"persian.jpg",
	"bengal.jpg",
	"ragdoll.jpg",
	"norweigen_forest.jpg",
}

var breeds = []gen.Suggestion{
	{Breed: "Sphinx", Reason: "hairless and judging you silently"},
	{Breed: "Scottish Fold", Reason: "folded ears, folded personality, entirely your problem now"},
	{Breed: "Maine Coon", Reason: "the size of a small dog but significantly more opinionated"},
	{Breed: "Siamese", Reason: "will talk at you until one of you gives up"},
	{Breed: "Persian", Reason: "requires more grooming than you, which is saying something"},
	{Breed: "Bengal", Reason: "beautiful chaos in a fur coat"},
	{Breed: "Ragdoll", Reason: "goes limp when held, which is more than most of us manage"},
	{Breed: "Norwegian Forest Cat", Reason: "could survive the apocalypse, barely tolerates you"},
}

func countVowels(name string) int {
	count := 0
	for _, c := range strings.ToLower(name) {
		switch c {
		case 'a', 'e', 'i', 'o', 'u':
			count++
		}
	}
	return count
}

type cattoServer struct{}

func (s *cattoServer) SuggestBreed(_ context.Context, req gen.SuggestBreedRequestObject) (gen.SuggestBreedResponseObject, error) {
	breed := breeds[countVowels(req.Params.Name)%len(breeds)]
	return gen.SuggestBreed200JSONResponse(breed), nil
}
