# Product

<!-- impeccable:product-schema 1 -->

## Platform

adaptive

## Users

Kite serves developers working inside a repository who want to run, observe,
and resume an agent without leaving the terminal. Library consumers also embed
the same runtime beneath their own supervisors and interfaces.

## Product Purpose

Kite is community-built infrastructure for reliable coding agents. It combines
a small embeddable Go runtime with a CLI that makes model turns, tool activity,
durable sessions, verification, and artifacts understandable and recoverable.

## Positioning

Kite keeps the runtime small and open while treating every important agent
transition as durable, inspectable state. The terminal interface is a view over
that runtime, not a second execution engine.

## Operating Context

People use Kite from Linux, macOS, and Windows terminals while working in a
repository. Runs may be long, may modify files, may produce large artifacts,
and may be interrupted or resumed. Existing non-interactive CLI, RPC, and Go
API workflows remain first-class.

## Capabilities and Constraints

- Runtime and documentation tooling remain Go standard-library only.
- The TUI must work across release targets and degrade cleanly without ANSI
  color or an interactive terminal.
- Existing versioned event, result, and RPC v1 contracts remain compatible.
- Credentials never appear in interface output or persisted errors.
- Three selectable color themes are required, with meaning never conveyed by
  color alone.
- Community extension areas include providers, tools, executors, policies,
  examples, integrations, documentation, and compatibility tests.

## Brand Commitments

The product name is Kite. Its voice is concise, capable, transparent, and
calm under failure. The visual identity must feel native to serious terminal
work without falling into generic neon-hacker styling.

## Evidence on Hand

The repository contains the working runtime, executable CLI and RPC surfaces,
behavioral tests, versioned schemas, agent documentation, and compiled examples.
There are no external brand assets or commercial claims to introduce.

## Product Principles

- Keep the core small; make extension seams explicit.
- Persist before presenting so recovery is trustworthy.
- Show what the agent is doing without overwhelming the operator.
- Prefer observable behavior and executable contracts over implied guarantees.
- Preserve a useful plain-text path everywhere.

## Accessibility & Inclusion

The terminal interface supports keyboard-only operation, plain-text and
`NO_COLOR` output, readable contrast, explicit status labels, and layouts that
remain understandable when color and decorative glyphs are unavailable.
