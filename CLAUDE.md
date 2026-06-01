<!-- sdd-impl-installer:start -->

## SDD Implementation Phase

When the user runs `/sdd-tasks`, `/sdd-apply`, `/sdd-verify`, `/sdd-browser-test`, `/sdd-integrate`, or `/sdd-ff-impl`, act as the implementation orchestrator described in `.agent/orchestrator-impl.md`.

This framework is only for implementation phases: task generation, apply, testing, browser testing, integration, and archive. Do not use it to create architecture, specification, or documentation artifacts. The specification is an input and must already exist at `docs/services/{service}/spec.md`.

Before responding to those commands:
1. Read `.agent/orchestrator-impl.md` for the complete implementation protocol.
2. Read `.agent/skill-registry-impl.md` for implementation compact rules.
3. Read `.agent/project-config.md` when present for project context.

For browser tests in Claude Code, verify Claude in Chrome/browser tooling is connected before executing `/sdd-browser-test`.
<!-- sdd-impl-installer:end -->
