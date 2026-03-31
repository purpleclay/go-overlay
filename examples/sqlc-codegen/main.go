package main

import (
	"context"
	"database/sql"
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/purpleclay/go-overlay/examples/sqlc-codegen/gen"

	_ "modernc.org/sqlite"
)

//go:embed db/schema.sql
var schema string

var excuses = []gen.SeedExcuseParams{
	// build
	{Category: "build", Body: "It compiles on my machine."},
	{Category: "build", Body: "The build cache must be corrupted."},
	{Category: "build", Body: "Someone pushed a bad dependency."},
	{Category: "build", Body: "The linker is being temperamental today."},
	{Category: "build", Body: "Nix is rebuilding the universe again."},
	// tests
	{Category: "tests", Body: "The tests were already failing before I touched it."},
	{Category: "tests", Body: "That test is flaky, everyone knows it."},
	{Category: "tests", Body: "I was going to write tests but ran out of time."},
	{Category: "tests", Body: "The test environment is different from production."},
	{Category: "tests", Body: "Those are integration tests, they don't count."},
	// deploy
	{Category: "deploy", Body: "It works in staging."},
	{Category: "deploy", Body: "The rollback also failed, so it's fine."},
	{Category: "deploy", Body: "We just need to restart the pods."},
	{Category: "deploy", Body: "The load balancer is doing something weird."},
	{Category: "deploy", Body: "It's a DNS issue, it'll resolve itself."},
	// review
	{Category: "review", Body: "I left a comment but nobody replied."},
	{Category: "review", Body: "The PR has been approved, I'm just waiting for the pipeline."},
	{Category: "review", Body: "It's a draft, I wasn't ready for feedback yet."},
	{Category: "review", Body: "The reviewer is on holiday."},
	{Category: "review", Body: "It's only a two-line change."},
	// meeting
	{Category: "meeting", Body: "I thought it was optional."},
	{Category: "meeting", Body: "My calendar didn't sync."},
	{Category: "meeting", Body: "I was in another meeting about the meeting."},
	{Category: "meeting", Body: "I didn't get the invite."},
	{Category: "meeting", Body: "I was heads-down and lost track of time."},
}

func main() {
	category := flag.String("category", "", "filter by category: build, tests, deploy, review, meeting")
	flag.Parse()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()

	if _, err := db.ExecContext(ctx, schema); err != nil {
		log.Fatal(err)
	}

	q := gen.New(db)

	for _, e := range excuses {
		if err := q.SeedExcuse(ctx, e); err != nil {
			log.Fatal(err)
		}
	}

	var cat interface{}
	if *category != "" {
		cat = *category
	}

	excuse, err := q.RandomExcuse(ctx, cat)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no excuses found — try a different -category")
		os.Exit(1)
	}

	if err := q.IncrementTimesUsed(ctx, excuse.ID); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("[%s] %s\n", excuse.Category, excuse.Body)
}
