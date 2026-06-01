<!-- sdd-impl-installer:start -->

## SDD Implementation Phase

This project has an SDD implementation system installed under `.agent/`.

This framework is only for implementation phases: task generation, apply, testing, browser testing, integration, and archive. It does not install or run architecture, specification, or documentation agents.

The service specification is an input and must already exist at:
`docs/services/{service}/spec.md`

Use the local Codex skill generated at:
`.agent/codex-skills/axis-flow-sdd-impl-orchestrator/SKILL.md`

Implementation commands:
- `/sdd-tasks <service>`
- `/sdd-apply <service> [--tdd|--no-tdd]`
- `/sdd-verify <service>`
- `/sdd-browser-test <service> <url>`
- `/sdd-integrate <service>`
- `/sdd-ff-impl <service>`

Legacy source files:
- `.agent/orchestrator-impl.md`
- `.agent/skill-registry-impl.md`
- `.agent/sub-agents/*.md`
- `.agent/skills/*.md`

Codex mapping: legacy Claude sub-agents are role prompts. Use Codex worker agents or generated local skills from `.agent/codex-skills/` and inject the matching role instructions when delegating implementation work.
<!-- sdd-impl-installer:end -->
