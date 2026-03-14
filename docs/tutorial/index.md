# Tutorial: Building a Polls App

This hands-on tutorial walks you through building a complete **polls application** with Burrow — from an empty directory to a fully-featured web app with authentication, an admin panel, and HTMX-powered interactivity.

## What You'll Build

A survey/voting application where users can:

- Browse published questions
- Vote on choices
- View results with charts
- Manage questions and choices via an admin panel

## Prerequisites

- **Go 1.26+** installed
- Basic familiarity with Go (functions, structs, interfaces)
- A text editor and terminal

## Parts

| Part | Topic | What You'll Learn |
|------|-------|-------------------|
| [Part 1](part1.md) | Setup & First View | Project scaffolding, server lifecycle, HandlerFunc |
| [Part 2](part2.md) | Database & Models | App interface, Bun/SQLite, migrations |
| [Part 3](part3.md) | Templates & Layouts | Template system, layouts, Render |
| [Part 4](part4.md) | Forms, CRUD & Validation | Form handling, CSRF, messages |
| [Part 5](part5.md) | Authentication | Auth system, middleware, user context |
| [Part 6](part6.md) | Admin Panel | ModelAdmin, HasAdmin interface |
| [Part 7](part7.md) | HTMX, Charts & Polish | htmx helpers, i18n, pagination |

Each part builds on the previous one. The complete source code for each step lives in the [`tutorial/`](https://github.com/oliverandrich/burrow/tree/main/tutorial) directory.

## How to Follow Along

You can either:

1. **Type the code yourself** — follow the walkthrough and create each file as described
2. **Read the source** — each step has a complete, compilable project in `tutorial/stepNN/`

To run any step:

```bash
cd tutorial/step01
go run .
```

The server starts on `http://localhost:8080` by default.
