# Prompt 01b: Supabase Database Integration For MVP Start

You are a senior Go engineer. Integrate the MVP with a managed PostgreSQL database using Supabase as the starting provider.

## Goal
Use Supabase PostgreSQL on the free tier to get the MVP running quickly, while keeping the codebase portable to any normal PostgreSQL host later.

## Important Constraint
Treat Supabase as a hosting choice, not as a reason to redesign the application around Supabase-specific auth, edge functions, or client SDK patterns.

Keep the service architecture exactly as defined in `00_source_of_truth.md`:
- one Go runtime service
- direct PostgreSQL access from the backend
- server-rendered admin pages
- straightforward SQL and migrations

## What To Build In This Step
- document the expected local and hosted database flow using Supabase
- make sure the app uses a standard `DATABASE_URL`
- confirm the Go service connects with a normal PostgreSQL driver
- keep migrations runnable against Supabase Postgres without provider-specific tooling requirements
- note any connection or SSL expectations that matter for production readiness

## Supabase Start Rules
- start with Supabase free tier for cost control
- use the database only from the backend service, never directly from Telegram clients or browsers
- prefer standard PostgreSQL capabilities that will also work outside Supabase
- do not couple the MVP to Supabase Auth, Realtime, Storage, or Edge Functions unless a later prompt explicitly asks for them
- assume the free tier is good enough for early MVP validation, but keep the exit path simple

## Config Expectations
At minimum, the implementation and docs should support:
- `DATABASE_URL` as the primary connection string
- local `.env` usage for development
- clear guidance on where the Supabase connection string is supplied

## Simplicity Rules
- no ORM
- no Supabase client SDK in the Go backend if plain PostgreSQL access is enough
- no extra database proxy layer unless required by deployment constraints
- no multi-database setup for the MVP

## Acceptance Criteria
- the repo documents Supabase as the initial managed PostgreSQL provider
- the app still depends only on standard PostgreSQL connectivity through `DATABASE_URL`
- migrations and repository code remain portable PostgreSQL code
- the path to upgrade from free tier later is operational, not architectural
