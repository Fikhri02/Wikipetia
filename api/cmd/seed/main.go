// Command seed creates one small course so manual testing and demos do not
// start with twenty minutes of clicking. Safe to run repeatedly: it makes a new
// course each time.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/google/uuid"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/enrolment"
	"github.com/Fikhri02/wikipetia/internal/platform/config"
	"github.com/Fikhri02/wikipetia/internal/platform/db"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
	"github.com/Fikhri02/wikipetia/internal/user"
)

func main() {
	if err := run(); err != nil {
		slog.Error("seed", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	runner := db.PoolRunner{Pool: pool}
	store := user.Store{}

	var owner uuid.UUID
	if err := runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		var err error
		owner, err = store.EnsureBySubject(ctx, q, "dev|dev", "dev")
		return err
	}); err != nil {
		return err
	}

	courses := course.New(runner)

	c, err := courses.Create(ctx, owner, course.CreateInput{
		Title:       "Form 4 Chemistry — seed",
		Description: "A seeded course for manual testing.",
		PetEnabled:  true,
	})
	if err != nil {
		return err
	}

	atoms, err := courses.CreateTopic(ctx, owner, c.ID, "Atomic structure")
	if err != nil {
		return err
	}
	bonding, err := courses.CreateTopic(ctx, owner, c.ID, "Chemical bonding")
	if err != nil {
		return err
	}

	mod, err := courses.CreateModule(ctx, owner, c.ID, "Module 1 — Matter")
	if err != nil {
		return err
	}

	reading, err := courses.CreateBlock(ctx, owner, mod.ID, course.CreateBlockInput{
		Title:   "What is an atom?",
		Type:    course.BlockReading,
		Content: json.RawMessage(`{"body_md":"# Atoms\n\nAn atom is the smallest unit of an element."}`),
	})
	if err != nil {
		return err
	}
	if err := courses.SetBlockTopics(ctx, owner, reading.ID, []uuid.UUID{atoms.ID}); err != nil {
		return err
	}

	video, err := courses.CreateBlock(ctx, owner, mod.ID, course.CreateBlockInput{
		Title:   "Atomic structure explained",
		Type:    course.BlockVideoYouTube,
		Content: json.RawMessage(`{"video_id":"FSyAehMdpyI"}`),
	})
	if err != nil {
		return err
	}
	if err := courses.SetBlockTopics(ctx, owner, video.ID, []uuid.UUID{atoms.ID}); err != nil {
		return err
	}

	quiz, err := courses.CreateBlock(ctx, owner, mod.ID, course.CreateBlockInput{
		Title: "Check yourself",
		Type:  course.BlockQuiz,
	})
	if err != nil {
		return err
	}
	if err := courses.SetBlockTopics(ctx, owner, quiz.ID, []uuid.UUID{atoms.ID, bonding.ID}); err != nil {
		return err
	}

	questions := []course.QuestionInput{
		{
			Kind: course.QuestionMCQSingle, TopicID: &atoms.ID,
			Prompt: json.RawMessage(`{"text":"Which of these is an element?","options":["Water","Iron","Salt"]}`),
			Answer: json.RawMessage(`{"correct_index":1}`),
		},
		{
			Kind: course.QuestionTrueFalse, TopicID: &atoms.ID,
			Prompt: json.RawMessage(`{"text":"An atom is smaller than a molecule of water."}`),
			Answer: json.RawMessage(`{"correct":true}`),
		},
		{
			Kind: course.QuestionTyped, TopicID: &bonding.ID,
			Prompt: json.RawMessage(`{"text":"What is the chemical symbol for iron?"}`),
			Answer: json.RawMessage(`{"accepted":["Fe"],"case_sensitive":false}`),
		},
	}
	for _, in := range questions {
		if _, err := courses.CreateQuestion(ctx, owner, quiz.ID, in); err != nil {
			return err
		}
	}

	e, err := enrolment.New(runner).Enrol(ctx, owner, c.ID, enrolment.ModeSelfStudy)
	if err != nil {
		return err
	}

	fmt.Printf("seeded\n  course     %s\n  enrolment  %s\n  reading    %s\n  video      %s\n  quiz       %s\n",
		c.ID, e.ID, reading.ID, video.ID, quiz.ID)
	return nil
}
