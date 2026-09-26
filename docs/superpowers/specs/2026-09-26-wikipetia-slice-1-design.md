# WikiPetia — Slice 1 Design

**Date:** 2026-09-26
**Status:** Design approved, spec pending review
**Scope of this spec:** Slice 1 only — authoring, self-study consumption, and the topic/mastery layer.
Later slices get their own specs.

---

## 1. Context

WikiPetia is a study platform where learners take courses either on their own (self-study) or inside a
teacher-run class. Courses are built from content blocks (video, reading, quiz, flashcards, and more).
Progress is tracked by mastery rather than completion, and learners raise a study pet per course that
becomes a collectible card on completion.

The product structure is described in full in
[Study App Project Structure](https://claude.ai/code/artifact/7ee60f2c-0e12-4894-aef9-7d885be969cd).
That document is the product reference; this spec is the engineering design for the first slice of it.

The core product idea, which the architecture must preserve: **separate what is taught (the Course) from
how it is run (self-study, or a Class with a teacher).** One course powers many classes and any number
of self-study learners.

### Framing decisions

| Decision | Value | Why |
|---|---|---|
| Ambition | Real product, web first | The course builder is desktop work regardless of what learners use, so web is one client instead of two. YouTube's IFrame Player API — needed to verify video completion, the top technical risk — is a browser API. |
| First user | The author himself | Authoring speed and the self-study learning loop matter most; classroom features wait until a second person needs them. |
| Platform | Responsive web, installable PWA | Learners get a phone-usable experience without a second client. An Expo learner app stays open as a later addition. |

**Consequence of "first user is the author":** PDPA and under-18 parental consent leave the critical path.
No other person's data exists until access control ships (slice 5), so compliance becomes a gate on
*inviting real users*, not on building. This must be re-raised before slice 5.

---

## 2. Decomposition and build order

The product reference is not one project. It describes at least six subsystems, each large enough to
carry its own spec:

1. Course authoring — modules, 18 block types, content storage
2. Classroom / LMS — classes, rosters, groups, assignments, drip release, checkpoint gates
3. Identity & access — visibility vs enrolment, invitations, one-time join codes
4. Learning engine — spaced repetition, mastery scoring, adaptive practice
5. Progress & analytics — dual progress bars, streaks, 12-month heatmap
6. Engagement — study pets, garden, trading cards, mascot crew

### Build order

| Order | Slice | Rationale |
|---|---|---|
| **1** | **Authoring + self-study consumption + topics** | Nothing exists without content and a learner who can complete it. **This spec.** |
| 2 | Learning engine — flashcards, spaced repetition, per-topic mastery, checkpoints | Promoted from the reference's Phase 3: this is where the first user gets value, and it is what makes mastery honest |
| 3 | Progress & analytics — dual bars, streaks, heatmap | Reads from the engine |
| 4 | Engagement — pets, garden, evolution, cards | Opt-in per course, event-driven, deletable |
| 5 | Identity & access — visibility, invitations, join codes | Demoted: join codes are unnecessary while there is one user. Gate for PDPA work. |
| 6 | Classroom — classes, roster, access control, assignments, groups | Demoted: build when a second person needs it |

Two deliberate departures from the product reference's own phasing:

- **Flashcards move out of Phase 1.** Flashcards without a spaced-repetition engine are a card viewer, and
  the pet's health signal comes from that engine. They ship with slice 2, alongside the engine they need.
- **No standalone video-verification spike.** The Google Drive playback risk applies only to the Drive
  video block, which is not in slice 1. Slice 1 ships YouTube only, whose end-detection is well-trodden.
  The Drive spike happens immediately before Drive video is scheduled. Slice 1's first task includes a
  short verification of YouTube player events.

**The classroom system — which the product reference treats as the core product — is built last.** The
design must therefore never let slices 1–4 assume a single-learner world. The seam is the **enrolment**:
every progress row hangs off an enrolment that already carries a mode and an optional class, so classes
slot in later without touching progress.

---

## 3. Product decisions banked during design

### Topics exist from slice 1

The product reference measures mastery per *skill* ("skills reaching proficient", "groups by shared weak
topics", misconception clustering) but its entity list has no skill or topic. Without a topic layer,
mastery can only ever mean "you scored above threshold on this specific quiz", the heatmap cannot show
*what* is fading, and weak-topic grouping is unimplementable. Retrofitting it later means rewriting
progress, pets and analytics at once.

**Decision:** a Topic entity is in the core model from slice 1. Blocks and individual quiz questions are
tagged to topics, and mastery is computed per topic.

### The engagement layer is opt-in and loosely coupled

**Decision:** pets are **opt-in per course** — a course carries a flag, and if the creator leaves it off,
enrolments in that course hatch nothing and show no pet UI. **One pet per enrolment.** Active pets live
in the learner's **garden** (this replaces the product reference's "habitat"); a completed pet becomes a
card in the learner's collection.

**Architectural expression of that coupling:** the engagement subsystem *subscribes to learning events and
is never read from by the learning core.* Learning emits facts — `block.completed`, `reviews.completed`,
`topic.mastered`, `rest.day`, `course.completed` — into an append-only event log; the engagement module
consumes them and keeps its own state. No learning package may import engagement code. The whole layer
can therefore be built last, switched off per course, or deleted as a directory.

---

## 4. Technology decisions

| Area | Choice | Notes |
|---|---|---|
| API | **Go** — `huma` router, `pgx`, `sqlc`, `golang-migrate` | Hand-written SQL suits the aggregate-heavy queries; background jobs are goroutines rather than cron routes; an events-table consumer is idiomatic; a static binary is cheap to run |
| Database | **PostgreSQL** (Docker locally) | `jsonb` for polymorphic blocks, window functions for streaks, `generate_series` for the heatmap, `pgvector` available later. sqlc's Postgres/pgx path is its most mature |
| Front end | **Vite + React + TypeScript**, TanStack Query + Router, Tailwind, `vite-plugin-pwa` | Behind a login, SSR buys little; a static SPA has no server to host |
| Contract | **OpenAPI generated from Go, then `openapi-typescript` + `openapi-fetch`** | See below |
| Auth (slice 1) | **Dev stub behind a Go interface** | See below |
| Hosting (slice 1) | **None — local only** | See below |

### The contract must be generated, not written twice

A Go API plus a TypeScript front end has no compile-time agreement about JSON shapes. Writing the shape
in both places means a renamed Go field compiles fine, passes tests, and renders `NaN%` in the browser —
contract drift that accumulates silently across endpoints.

**Decision:** Go structs are the single source of truth. `huma` derives `openapi.yaml` from them;
`openapi-typescript` generates `web/src/api/types.ts`; components import only generated types; CI fails a
push where the spec or client is stale. This restores compile-time safety at the boundary.

Two limits of that contract, both verified by generating against the real spec rather than assumed:

- **`TypeScript` must stay on 5.x.** `openapi-typescript@7` declares `peer typescript@"^5.x"` and refuses
  to install against TypeScript 6. The Vite scaffold defaults to 6.x, so the web setup pins 5.9.3
  deliberately.
- **Polymorphic `jsonb` fields arrive as `unknown`.** `blocks.content`, `blocks.completion` and
  `questions.prompt` are `json.RawMessage` in Go, so the generated client gives them no shape. That is
  honest — they genuinely vary by block type — but it means the web block registry gets no type safety
  from the contract and must parse them at runtime, with one parser per block type mirroring the Go
  `blockSpec` registry. Arrays likewise generate as `T[] | null`, because Go slices are nilable.

### Local-first, and auth deferred

Nothing in slice 1 needs to be reachable from the internet. Postgres runs in one Docker container, the
connection string lives in config, and choosing a host later is an env-var change rather than a migration.

Auth is the only piece that is not naturally local. **Decision:** auth sits behind a Go interface whose
job is to put a user ID on the request context. In development it reads a header or a configured default
user; a real JWT-verifying implementation slots in behind the same interface later. Every domain package
keys off our own `users` row, so the identity provider stays swappable.

**Risk this accepts:** the auth integration is untested until later, and integrations are where surprises
live. Mitigation: it is small (JWT verification is ~30 lines, provider swappable by design), and the real
provider must be adopted **before slice 3**, because that slice's value depends on daily use, which means
wanting the app on a phone. The moment it leaves the laptop, auth must be real.

### Deferred infrastructure decision (trigger: end of slice 2)

When hosting is needed, the intended shape is **Supabase for Postgres + Auth + Storage** (one service
rather than three, and storage is needed by later slices for reading-block images and file-upload blocks),
**Go API on Fly.io** with scale-to-zero, **static web on Cloudflare Pages**. Approximate cost: $0–3/month
at single-user scale, ~$3–5 as a real product.

Two rules to carry into that decision, both learned from the staylokal architecture review:

1. **Supabase would be infrastructure, not our API.** Postgres via `pgx`/`sqlc`, auth as a JWT issuer,
   storage as blobs. No PostgREST, no `supabase-js` data queries, no Realtime. The React app talks to the
   Go API for every piece of data and to Supabase only to log in. One data path.
2. **Authorization lives in Go; RLS stays off.** The API enforces access in the domain layer, where it is
   testable with plain Go tests and can express "enrolled, or the owner, or a teacher of a class on this
   course". Duplicating it in RLS policies would create two sources of truth that drift — the exact
   failure mode that review found. Trade-off accepted: a leaked connection string has no second line of
   defence, so the service key lives only in deployment secrets and never in the web bundle.

---

## 5. Architecture

One repository, two deployables.

```
wikipetia/
  api/                    Go, one binary
    cmd/api/              main: wiring, config, HTTP server, handlers
    internal/
      course/             courses, modules, blocks, topics, questions
      enrolment/          enrolments; the seam classes slot into later
      progress/           block progress, per-topic mastery, completion evaluators
      srs/                (slice 2) scheduling, review queue
      engagement/         (slice 4) pets, garden, cards — event consumer only
      events/             append-only event log + dispatch
      platform/           db, config, auth middleware, http helpers
    db/migrations/        golang-migrate SQL
    db/queries/           sqlc sources -> generated Go
    openapi.yaml          generated from huma, committed
  web/                    Vite + React + TS, static build
    src/api/              generated types + thin fetch wrapper
    src/features/         library, course-builder, course-player
    src/blocks/           one component per block type, via a registry
    src/components/       shared UI
  docs/superpowers/       specs and plans
  docker-compose.yml      Postgres only
  Makefile                generate, migrate, test, run, seed
```

Go module path: `github.com/Fikhri02/wikipetia`.

### Three boundary rules that make the modules real

1. **Domain packages never import HTTP.** `internal/course` knows nothing of huma, requests or JSON.
   Handlers in `cmd/api` translate HTTP into domain calls. This is what lets an Expo client, a CLI or a
   background job reuse every piece of logic unchanged.
2. **`engagement` is a consumer, never a dependency.** Nothing in `course`, `enrolment`, `progress` or
   `srs` may import `engagement`. Enforced by a **lint rule in CI**, not by good intentions, because this
   is the boundary that erodes first.
3. **All SQL lives in `db/queries` as sqlc sources.** No query strings scattered through Go. The aggregate
   queries — mastery rollups, due queues, heatmap buckets — are the product's real logic and belong in one
   readable place.

### Deliberately absent from slice 1

No Redis. No message broker — the event log is a Postgres table, which is honest and sufficient at this
scale. No microservices, no GraphQL. No file uploads (YouTube links and typed text only), so no object
storage. No deployment.

---

## 6. Data model (slice 1)

```
users            id, auth_subject (unique, nullable until real auth), name, created_at

courses          id, owner_id -> users, title, description,
                 visibility ('public'|'unlisted'|'private'), pet_enabled bool,
                 created_at, archived_at

topics           id, course_id -> courses, title, sort_key, archived_at

modules          id, course_id -> courses, title, sort_key, archived_at

blocks           id, module_id -> modules, title, type, sort_key,
                 content jsonb, completion jsonb, archived_at

block_topics     block_id, topic_id                        (composite PK)

questions        id, block_id -> blocks, topic_id -> topics (nullable),
                 kind, prompt jsonb, answer jsonb, sort_key, archived_at

enrolments       id, user_id, course_id, mode ('self_study'|'classroom'),
                 class_id null, started_at, completed_at
                 unique (user_id, course_id) WHERE class_id IS NULL
                 unique (user_id, course_id, class_id) WHERE class_id IS NOT NULL

block_progress   enrolment_id, block_id, status, score, attempts,
                 completed_at, updated_at                  (composite PK)

topic_mastery    enrolment_id, topic_id, strength numeric, evidence_count,
                 last_evidence_at, updated_at              (composite PK)

events           id bigserial, occurred_at, user_id, enrolment_id null,
                 kind, payload jsonb, consumed_at null
```

### Decisions embedded in that model

**Topics are scoped to a course, not global.** Simpler to author, and every feature needing them operates
inside one course. Cost: "you are weak on stoichiometry" cannot span two chemistry courses. Going global
later is additive (a global topic table plus a mapping), not a migration of progress data.

**Blocks are polymorphic via `content jsonb` + a `type` discriminator**, not a table per type. With 18
block types coming, per-type tables means 18 joins and a migration per new type. Trade-off: Postgres
cannot enforce a video block's shape, so Go validates against a per-type struct on write — which is the
right place for it, since completion rules are behaviour rather than constraints.

**`completion jsonb` per block**, because every block type in the reference has its own rule (watched to
end, score at or above threshold, N correct at difficulty). Storing the rule with the block lets creators
set thresholds per block and lets `progress` evaluate completion without a type switch in three places.

**All progress hangs off `enrolment_id`, never `user_id`.** This is the seam that lets the classroom
arrive last: a class enrolment is an enrolment with `class_id` set and `mode = 'classroom'`, and nothing
in `block_progress` or `topic_mastery` changes when slice 6 lands. The unique constraint including
`class_id` also answers one of the reference's open questions by construction — a learner **can** be in
self-study and a class for the same course, and they are separate progress records.

**Nothing is hard-deleted; `archived_at` everywhere.** Deleting a block that progress rows reference
orphans them and breaks every aggregate. Archiving keeps history honest at the cost of one
`WHERE archived_at IS NULL` in authoring queries.

**Ordering uses a lexicographic `sort_key` text column, not an integer position.** Dragging a module to
the top with integer positions rewrites every row in the course; with a fractional key it writes one.

**Units are fixed and not negotiable per call site.** `block_progress.score` is an integer 0–100
(percent), and `blocks.completion.threshold` is compared against it in the same units.
`topic_mastery.strength` is a numeric 0–1. Mixing these is the most likely source of a silent off-by-100
bug, so the spec fixes them here rather than leaving each evaluator to choose.

**`questions.topic_id` is nullable, and an untagged question still counts toward block completion but
contributes nothing to mastery.** Requiring a topic on every question would make the builder tedious and
push authors toward a junk "General" topic, which is worse than an honest null.

**`courses.visibility` exists in slice 1 but is not enforced.** The column defaults to `'private'` and is
written by the builder; the rules that make it mean something ship with access control in slice 5. This is
recorded so nobody assumes slice 1 protects anything — with one user and no auth, it does not.

**`topic_mastery.strength` is numeric with `last_evidence_at`; levels are thresholds computed on read**,
not stored. Mastery decays with time, so a stored level would be stale the moment nobody looks at it. In
slice 1 the only evidence is quiz scores (checkpoints arrive in slice 2); slice 2's SRS writes to
the same column.

---

## 7. API surface

### Authoring

```
POST   /courses                          GET   /courses                (mine)
GET    /courses/{id}/outline             PATCH /courses/{id}           POST /courses/{id}/archive
POST   /courses/{id}/topics              PATCH /topics/{id}            POST /topics/{id}/archive
POST   /courses/{id}/modules             PATCH /modules/{id}           POST /modules/{id}/archive
POST   /modules/{id}/blocks              PATCH /blocks/{id}            POST /blocks/{id}/archive
PUT    /blocks/{id}/topics               (set the whole tag list)
POST   /blocks/{id}/questions            PATCH /questions/{id}         POST /questions/{id}/archive
POST   /modules/{id}/move                POST  /blocks/{id}/move       {before|after: id}
```

### Learning

```
POST   /enrolments                                  {course_id, mode}
GET    /enrolments                                  (my library)
GET    /enrolments/{id}                             (everything the player needs)
POST   /enrolments/{id}/blocks/{bid}/complete       (reading, video)
POST   /enrolments/{id}/blocks/{bid}/submit         (quiz — server grades)
```

### Decisions in that surface

**Reordering is a `move` endpoint, not a client-computed `sort_key`.** The client says "put this after
that"; the server computes the fractional key. Ranking logic stays in one place and two tabs cannot mint
colliding keys.

**Quiz answer keys never leave the server.** `GET /enrolments/{id}` returns prompts and options but no
`answer`. Grading happens server-side in `POST .../submit`. Shipping answer keys to the browser defeats
every quiz in the product via devtools.

**One aggregate read per screen.** `GET /enrolments/{id}` returns the outline, every block's progress and
current topic mastery in a single response; `GET /courses/{id}/outline` does the same for the builder. A
SPA that fetches modules, then blocks per module, then progress per block is a waterfall that feels broken
on a phone.

**Writes are narrow.** `PATCH` takes only what changed. Topic tags are a `PUT` of the whole list, matching
how the UI works (a multi-select) and avoiding add/remove races.

---

## 8. Data flow — quiz submission

The pattern every other write follows.

```
POST /enrolments/42/blocks/915/submit   { answers: [...] }
  |
  |- auth middleware        -> user_id on request context
  |- handler (cmd/api)      -> decode, validate shape, call domain. No SQL here.
  |
  '- progress.SubmitQuiz(ctx, userID, enrolmentID, blockID, answers)
       |- load enrolment; reject unless enrolment.user_id == userID   <- authorization, in Go
       |- load block + questions (with answer keys - server side only)
       |
       '- ONE transaction:
            |- grade each answer                     -> per-question correct/incorrect
            |- upsert block_progress                    status, score, attempts+1,
            |                                           completed_at if score >= completion.threshold
            |- update topic_mastery per topic touched   strength, evidence_count, last_evidence_at
            |- append to events                         block.completed (only if newly complete),
            |                                           topic.mastered (only if threshold crossed)
            '- commit
  |
  '- response: score, per-question feedback, new block status, changed topic mastery
```

**Events are written in the same transaction as the state they describe** — the transactional outbox
pattern. This is what makes the loose coupling trustworthy: a pet can never react to a completion that was
rolled back, and a completion can never fail to emit its event. The consumer (slice 4) is a separate
goroutine polling `events WHERE consumed_at IS NULL`, so delivery is **at-least-once** and
**every engagement handler must be idempotent.** This constraint is recorded now, before any handler exists.

**Completion is evaluated in one place, driven by `blocks.completion`.** `progress` holds one evaluator per
block type behind a single interface. A `switch` on block type in the handler, the player and the progress
query is how the 18th block type becomes a three-day change.

**The mastery formula lives in one function**, so slice 2 can replace it. Slice 1 proposal: `strength` is
an exponentially weighted moving average of scores on that topic's questions, clamped to 0–1; levels are
thresholds applied on read (`< 0.5` learning, `0.5–0.8` proficient, `>= 0.8` mastered), with decay computed
from `last_evidence_at`. Crude — it does not model forgetting curves — but real, testable, and replaceable
behind the same column.

**Mark-complete is idempotent.** Posting `complete` twice returns the same state and emits no second event,
because events fire only on a status transition.

---

## 9. Front end

```
web/src/
  api/          generated types + thin fetch wrapper (auth header, error narrowing)
  features/
    library/          my enrolments, enrol in a course
    course-builder/   modules, blocks, topics, questions
    course-player/    the learning surface
  blocks/       one component per block type, resolved through a registry
  components/   shared UI
```

**TanStack Query is the only server-state store.** No Redux or Zustand for data that came from the API;
server data is a cache with invalidation, and treating it as global client state produces three
disagreeing copies of a course outline. Local UI state stays in components, with one deliberate
exception: **an in-progress quiz attempt is local until submit** — the server should not hear about
half-answered questions.

**The block registry mirrors the server's evaluator registry.** `blocks/` maps a block `type` to a React
component exactly as `progress` maps a type to a completion evaluator. This makes the extensibility claim
concrete: **adding a block type is one Go evaluator + one React component + one content schema**, touching
no switch statements, no player logic and no progress queries. If adding the fourth block type requires
editing the player, the boundary is wrong and gets fixed then rather than at the eighteenth.

**Optimistic updates in exactly two places:** drag-reorder in the builder and mark-complete in the player.
Both feel broken with a round-trip and both roll back cleanly. Everywhere else waits for the server —
unneeded optimism is just a second source of truth.

**Not in slice 1: offline support.** The PWA is installable so the app can go on a phone, but
service-worker content caching arrives with slice 2, where offline flashcards actually matter.

---

## 10. Error handling

1. **The domain returns sentinel errors; only one place knows HTTP.** `internal/*` returns `ErrNotFound`,
   `ErrForbidden`, `ErrConflict`, `ErrValidation` wrapping context. A single mapper in `cmd/api` turns
   them into status codes. No domain package imports `net/http`.
2. **Errors are `application/problem+json`**, emitted natively by huma and narrowed by the generated TS
   client. The exact shape, verified against huma v2.39.1 rather than assumed:
   `{$schema, title, status, detail, errors: [{message, location, value}]}` — note `location`
   (e.g. `body.title`), **not** `field`. The builder maps `location` onto form fields by stripping the
   `body.` prefix. **Shape-validation failures return 422, not 400**; `400` is reserved for malformed
   requests huma rejects before validation.
3. **Validation is two distinct layers.** Shape validation at the huma boundary via struct tags (is this a
   string, is it under 200 chars). Business rules in the domain (does this topic belong to this course, is
   the threshold between 0 and 100). Conflating them puts business rules in handlers, where they get
   duplicated.
4. **Unexpected errors never reach the client.** An unhandled error becomes a 500 with a correlation ID;
   the real error goes to structured logs (`slog`, request ID on every line); the client sees a generic
   message. Internal error strings leak schema and logic.
5. **The API never returns 200 with an error in the body.** Status codes carry the outcome, so TanStack
   Query's error path works without components inspecting response shapes.

---

## 11. Testing

| Layer | Covers | Against |
|---|---|---|
| Domain unit | Completion evaluators, mastery formula, `sort_key` ranking, quiz grading | Pure functions, no DB — fast, and there should be many |
| Repository | Every sqlc query: mastery rollup, outline read, upserts | A real Postgres with migrations applied |
| API | Each endpoint via `httptest` against the real router and a real DB | Status codes and `problem+json` shapes — the contract tests |
| Authorization | For every endpoint: a second user is rejected from the first user's data | Explicit per endpoint, not sampled |

**No mocking the database.** sqlc generates real SQL, so a mocked repository tests the mock. Repository
tests run against Postgres with a transaction rolled back per test — fast enough for the inner loop while
testing what actually ships. The domain layer is where mock-free unit testing happens, because it has no
DB in it by design.

**Authorization tests are a first-class layer.** Every endpoint gets a test where a second user tries to
touch the first user's data. This is not theoretical: the staylokal architecture review found that any
signed-in user could overwrite any host's property media, because authorization was assumed rather than
asserted. Being the only user is precisely what makes this easy to skip, so it is a required test per
endpoint in the plan.

**Front end:** Vitest + React Testing Library for block components and registry resolution; MSW for API
mocking with handlers typed from the generated OpenAPI types, so a drifted contract fails front-end tests
too. Plus **one** Playwright smoke test for the whole path — create course, add blocks, enrol, complete,
see progress. One, not a suite; broad E2E is slow and brittle and the API tests already cover the logic.

**CI gates on every push:** `go test ./...`, `go vet`, `sqlc diff`, OpenAPI regeneration diff, `tsc`,
`vitest`, and the **import-boundary check** failing the build if any learning package imports
`internal/engagement`. That rule is what makes the loose coupling structural rather than aspirational.

**A seed script** creates a small course so manual testing and demos do not start with twenty minutes of
clicking.

---

## 12. Definition of done

From an empty database, in a browser, the author can:

1. Create a course with two modules and four topics
2. Add a reading block, a YouTube video block, and a quiz block with five questions — each tagged to topics
3. Enrol in self-study mode
4. Mark the reading done; watch the video to the end and see it complete **automatically** via the player
   API; take the quiz and get a server-computed score with per-question feedback
5. See per-block completion and per-topic mastery reflect exactly what was done
6. Reload and find it all persisted
7. Inspect the `events` table and find `block.completed` and `topic.mastered` rows written in the same
   transactions

With all tests green and every CI gate passing.

### Explicitly out of scope for slice 1

Pets, garden, cards, mascots, flashcards, spaced repetition, streaks, heatmap, classes, groups,
assignments, join codes, invitations, file uploads, real auth, any deployment, the other fifteen block
types, and course versioning.

---

## 13. Deferred decisions, with triggers

| Decision | Trigger | Intended direction |
|---|---|---|
| Real auth provider | Before slice 3 — daily use means wanting it on a phone | Supabase Auth or Clerk issuing JWTs; ~30 lines of Go middleware behind the existing interface |
| Hosting and managed Postgres | End of slice 2 | Supabase (Postgres + Auth + Storage), Go on Fly with scale-to-zero, static web on Cloudflare Pages |
| Object storage | First block type needing uploads (reading images, file upload) | Supabase Storage, signed direct uploads |
| Course versioning | Before slice 5, when other people enrol | Publish drafts, pin enrolments to a version. See open question below |
| Global topics | If cross-course mastery is wanted | Additive: a global topic table plus a mapping |
| Google Drive video | Before that block type is scheduled | Spike first; fall back to mark-as-done plus questions, or direct upload |

---

## 14. Open questions

**Surfaced by this design:**

1. **Course editing versus in-flight progress.** Reordering modules or changing a quiz threshold while
   enrolled keeps progress rows valid (they key on block IDs), but a learner's completed count can move
   under them. Slice 1 accepts the wobble because there is one user. Real versioning is a subsystem and
   must be decided before slice 5.
2. **What mastery decay should actually look like.** Slice 1 uses a crude EWMA with time decay. Slice 2's
   SRS should replace it with a retention model, and the two must agree on the meaning of `strength`.

**Carried from the product reference, still open:**

3. Who can create courses: any user, verified teachers only, or organisations?
4. Should teachers be able to fork someone else's public course into their own class?
5. How should a pet behave for a learner with no active courses?
6. What happens to the pet if a learner drops a course before finishing it?
7. Minimum age and parental consent for under-18s (Malaysia PDPA) — gate on slice 5, not on building.

*Resolved by this design:* whether a learner can be in two classes for the same course — yes, and each is
a separate progress record, enforced by the pair of partial unique indexes in the data model.

---

## 15. Risks

| Risk | Mitigation |
|---|---|
| Auth integration untested until slice 3 | Provider swappable behind an interface; adopt before slice 3; JWT verification is small and well understood |
| Contract drift between Go and TypeScript | OpenAPI generated from Go structs, TS client generated from it, both checked in CI |
| `engagement` coupling erodes as features land | Import-boundary lint rule fails the build, not a code-review convention |
| Polymorphic `content jsonb` loses schema enforcement | Per-type Go structs validated on write; per-type tests; the shape is behaviour, not a constraint |
| Google Drive video cannot report playback | Out of slice 1 entirely; spike before scheduling; fall back to mark-as-done plus questions |
| YouTube videos removed or made private by their owners | Periodic link checks and an alert to the course owner — slice 2 or later, noted so it is not forgotten |
| Mastery model is crude and may mislead | One function, one column, replaceable; dual progress bars keep completion and mastery visibly distinct |
| Building six subsystems as one project | Decomposed above; each slice gets its own spec, plan and implementation cycle |
