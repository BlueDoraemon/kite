---
name: Kite
description: A durable terminal hunk ledger for supervising coding-agent sessions.
colors:
  night-flight-background: "#07111F"
  night-flight-text: "#D8E2F0"
  night-flight-accent: "#6EA8FF"
  night-flight-success: "#64D49B"
  night-flight-warning: "#F2B94B"
  night-flight-failure: "#FF6B6B"
  night-flight-muted: "#8290A4"
  paper-trail-background: "#F5F0E4"
  paper-trail-text: "#20252C"
  paper-trail-accent: "#245D96"
  paper-trail-success: "#347A45"
  paper-trail-warning: "#855700"
  paper-trail-failure: "#B23A3A"
  paper-trail-muted: "#5F6462"
  high-contrast-background: "#000000"
  high-contrast-text: "#FFFFFF"
  high-contrast-accent: "#00DCFF"
  high-contrast-success: "#50FF64"
  high-contrast-warning: "#FFE600"
  high-contrast-failure: "#FF4646"
  high-contrast-muted: "#BEBEBE"
typography:
  body:
    fontFamily: "monospace"
    fontSize: "1em"
    fontWeight: 400
    lineHeight: 1
    letterSpacing: "normal"
  label:
    fontFamily: "monospace"
    fontSize: "1em"
    fontWeight: 700
    lineHeight: 1
    letterSpacing: "normal"
rounded:
  square: "0"
spacing:
  cell: "1ch"
  row: "1lh"
components:
  ledger-row:
    backgroundColor: "{colors.night-flight-background}"
    textColor: "{colors.night-flight-text}"
    typography: "{typography.body}"
    rounded: "{rounded.square}"
    padding: "0"
  state-accent:
    textColor: "{colors.night-flight-accent}"
    typography: "{typography.label}"
    rounded: "{rounded.square}"
    padding: "0"
  state-success:
    textColor: "{colors.night-flight-success}"
    typography: "{typography.label}"
    rounded: "{rounded.square}"
    padding: "0"
  state-warning:
    textColor: "{colors.night-flight-warning}"
    typography: "{typography.label}"
    rounded: "{rounded.square}"
    padding: "0"
  state-failure:
    textColor: "{colors.night-flight-failure}"
    typography: "{typography.label}"
    rounded: "{rounded.square}"
    padding: "0"
  composer:
    backgroundColor: "{colors.night-flight-background}"
    textColor: "{colors.night-flight-accent}"
    typography: "{typography.label}"
    rounded: "{rounded.square}"
    padding: "0"
---

# Design System: Kite

## Overview

**Creative North Star: "The Durable Hunk Ledger"**

Kite presents one chronological record, shaped like source-control output rather than a dashboard. Durable event order is the navigation: prompts, turns, tool calls, artifacts, verification, results, and resumptions remain in one scan path. The interface is concise, capable, transparent, and calm under failure.

The system is terminal-native and information-dense. A single monospace size, sequence gutters, one-cell indentation, square geometry, and horizontal rules create hierarchy without ornamental containers. Color reinforces meaning but never owns it; every state remains legible through a literal marker or label.

**Key Characteristics:**

- One append-only chronological ledger rather than unrelated panels.
- Source-control hunk grammar for tool input, output, and result summaries.
- Three complete palettes with invariant textual state markers.
- Strict monospace typography at the terminal's configured size.
- Useful ANSI-free output with the same content order and labels.

## Colors

Three role-identical palettes change atmosphere without changing meaning or ledger structure.

### Primary

- **Night Flight Accent:** The default operational accent for tool starts, artifacts, resumptions, and the composer prompt.
- **Paper Trail Accent:** The restrained blue operational accent on a warm paper canvas.
- **High Contrast Accent:** The cyan operational accent for maximum separation on black.

### Secondary

- **Success:** Green reinforces `[ok]`, completed turns, and passed verification in every palette.
- **Warning:** Amber or yellow reinforces `[run]`, `[stop]`, and `[stale]` states.
- **Failure:** Red reinforces `[fail]` without replacing the explicit label.

### Neutral

- **Background:** Each theme paints one continuous terminal canvas; there are no nested surface colors.
- **Text:** The primary foreground carries event copy, paths, commands, and assistant output.
- **Muted:** Secondary metadata, preview gutters, command hints, repository context, and usage totals recede without disappearing.

### Named Rules

**The Marker Before Color Rule.** State must remain explicit in `[ok]`, `[fail]`, `[run]`, `[stop]`, `[stale]`, `[resume]`, `[file]`, or equivalent text before palette color is applied.

**The Complete Palette Rule.** A theme switch replaces background, text, accent, success, warning, failure, and muted roles together; never recolor a single state in isolation.

## Typography

**Display Font:** Terminal monospace
**Body Font:** Terminal monospace
**Label/Mono Font:** Terminal monospace

**Character:** The terminal owns the face and size. Kite uses regular and bold intensity within that one inherited monospace grid, keeping paths, commands, sequence numbers, previews, and prose aligned.

### Hierarchy

- **Body** (regular, one terminal em, one row): Event content, assistant text, paths, commands, previews, and result details.
- **Label** (bold, one terminal em, one row): Product identity, state markers, actor labels, section rules, tool labels, and the composer cue.

### Named Rules

**The One-Grid Rule.** Do not introduce a second font, proportional text, enlarged display type, or arbitrary tracking; hierarchy comes from weight, labels, rules, and spacing.

## Layout

The interface is one width-bounded stream. It defaults to 88 columns, respects `COLUMNS`, clamps to 48–120 columns, and truncates long single-line values with an ellipsis. The header establishes identity, session, model, theme, repository context, a full-width rule, and compact command help; the event ledger follows in durable sequence.

Every event row begins with a four-digit sequence number and one trailing cell. Tool previews sit under their start or finish row with a five-cell indent and a `|` gutter. Input previews show at most three lines and output previews at most four; longer content ends with an explicit inspection instruction. Section labels such as `PROMPT`, `RESULT`, and `CONTEXT` are embedded in full-width hyphen rules. The composer returns after the latest event as one blank row followed by the bold `> Ask Kite` cue.

**The Ledger Order Rule.** Never split durable events into cards, tabs, or competing columns. Sequence and proximity carry the operational story.

**The Bounded Preview Rule.** Keep the ledger scannable by truncating previews and pointing to the artifact or session for complete content.

## Elevation & Depth

The system is entirely flat. It uses no shadows, gradients, translucency, raised panels, or simulated elevation. Depth comes only from chronological nesting: sequence gutter, one-cell indentation, preview pipe, bold labels, muted metadata, and full-width rules.

**The Flat Ledger Rule.** A terminal row never becomes a floating surface; hierarchy remains structural and typographic.

## Shapes

All geometry is square and cell-aligned. Horizontal rules are one row high, preview boundaries use a single `|` glyph, and state markers use compact brackets or a leading plus. There are no rounded containers, pills, badges, or decorative frames in the shipped interface.

## Components

### Status Rail

A compact opening block identifies `KITE`, session, model, theme, and optional repository path. Product identity is bold, separators and field labels are muted, and values stay in the primary foreground. A full-width hyphen rule closes the rail before the command hint.

### Event Ledger Row

Each durable row uses a zero-padded four-digit sequence gutter, then a literal state or actor marker and concise content. Assistant streaming text aligns under a five-cell indent with the bold `KITE |` actor label. Unknown events remain visible as muted `[event]` rows rather than disappearing.

### Tool Hunk

A tool begins with the accent `+ TOOL` marker and its name. Up to three input lines appear beneath `| input`; completion repeats the tool name with `[ok]` or `[fail]` and shows up to four output lines beneath `|`. Truncation is explicit and directs the operator to durable full content.

### Result Seal

The `RESULT` rule closes a run into a compact summary: explicit completion state, changed files, verification status including staleness, and total token usage. Passed verification is `[ok]`; stale verification remains textually `[stale]` even when warning color is unavailable.

### Composer

The composer is an inline terminal prompt, not a web field or floating panel. It begins after one blank row with the accent, bold `> Ask Kite` cue and accepts one buffered line. Slash commands use the same stream and return explicit feedback.

### Plain Fallback

When ANSI is unavailable, output is redirected, `NO_COLOR` is set, `TERM=dumb`, or Windows terminal support is absent, Kite removes ANSI styling but preserves text, spacing, sequence order, markers, rules, and composer wording. A clear request becomes a `CLEARED` ledger rule when destructive screen repainting is unavailable.

### Theme Switching

`/theme` validates a stable theme name or alias, repaints only subsequent ANSI output, and writes `[ok] theme set to …`. It does not erase or redraw the ledger, preserving the operator's durable navigation context.

### Control-Byte Safety

Model, tool, path, and event content is sanitized before rendering. Newlines remain structural, tabs become one space, C0/C1 controls and bidirectional formatting controls are removed, and display-width truncation accounts for combining and wide characters.

## Do's and Don'ts

### Do:

- **Do** preserve one chronological ledger with four-digit sequence gutters and explicit state labels.
- **Do** use the active theme's complete seven-role palette and keep every state understandable without color.
- **Do** retain one-cell indentation, square rules, bounded previews, and terminal-owned monospace typography.
- **Do** sanitize untrusted content before it reaches ANSI rendering and preserve structural newlines.
- **Do** keep theme changes non-destructive so earlier ledger rows remain available as navigation.

### Don't:

- **Don't** turn events, tools, or results into unrelated cards, dashboard columns, tabs, pills, or rounded web controls.
- **Don't** use color, icons, or animation as the only indication of running, success, warning, failure, interruption, or staleness.
- **Don't** add shadows, elevation, gradients, ornamental borders, or a second typography scale.
- **Don't** print unbounded tool output into the live ledger when durable artifact or session inspection is available.
- **Don't** let model or tool content emit terminal controls or bidirectional overrides.
