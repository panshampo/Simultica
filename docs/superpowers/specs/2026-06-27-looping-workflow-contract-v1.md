# Simultica Looping Workflow Contract v1

## Goal

Simultica and agent-runtime must both treat skill workflows as first-class executable definitions that can branch, loop, stop safely, and explain their runtime decisions on issue workflow runs.

## Problem

Today a skill can store `workflow.yaml`, and an issue can start that workflow through the sidecar. The runtime already supports basic conditional edges and loop-back routes, but the platform contract is implicit:

- skill save accepts almost any `workflow.yaml` text;
- condition expressions are not validated against declared state;
- loop-back edges are not checked for an execution budget;
- runtime progress shows node status but not branch decisions;
- budget exhaustion and human-gated continuation are workflow conventions, not platform semantics.

This makes advanced workflows possible but easy to misconfigure.

## Contract

### Definition Shape

A workflow definition is stored in a skill as `workflow.yaml` and is snapshotted into `workflow_run.definition_snapshot` when an issue starts it.

Required top-level fields:

- `meta`: `{ name, version?, description? }`
- `state.fields`: named workflow state fields
- `nodes`: executable nodes
- `routing`: directed edges
- `execution`: execution policy

Supported advanced routing:

```yaml
routing:
  - from: review
    condition: 'workflow_status == "fixable_auto" && revisionCount < 3'
    to: stabilize
    else: final_report
```

Supported loop pattern:

```yaml
nodes:
  - id: review
    type: agent
    on_complete:
      - action: increment
        field: revisionCount

execution:
  maxSteps: 12
```

### Validation Rules

The runtime validator must reject:

- condition syntax errors;
- condition references to fields not declared in `state.fields`;
- conditional routes without `else`;
- loop-back routes without `execution.maxSteps`;
- `on_complete.increment` fields that are missing from `state.fields` or are not `number`.

The validator may warn for:

- any loop-back route whose condition does not reference a numeric counter;
- condition routes whose `to` and `else` point to the same target;
- workflows with no terminal route to `END`.

### Runtime State Convention

Looping workflows should use these shared state fields:

- `workflow_status`: `pending | done | fixable_auto | fixable_needs_decision | blocked | budget_exhausted`
- `next_target`: `author_case | stabilize_run | final_report | <node id>`
- `blocker_type`: `none | source_data_drift | empty_data | auth_runtime | mutation_policy | product_expectation_mismatch | ppe_not_deployed | scm_parity_failed | unknown`
- `needs_user_decision`: boolean
- `revisionCount`: number
- `evidence_summary`: string
- `next_action`: string

`revisionCount` counts completed review cycles, not individual node executions.

### Runtime Progress Convention

`workflow_run.nodes_state` must explain routing decisions in addition to node status. For a node with conditional routing, the source node state should include:

```json
{
  "route_decision": {
    "condition": "workflow_status == \"fixable_auto\" && revisionCount < 3",
    "condition_result": true,
    "selected_route": "stabilize_run",
    "else_route": "final_report",
    "decided_at": "2026-06-27T00:00:00.000Z"
  }
}
```

This makes issue runtime UI able to show why a branch was chosen or why a loop stopped.

### Human Gate And Continuation

The first platform-safe version does not need true in-place graph resume. When a workflow stops with `fixable_needs_decision` or `budget_exhausted`, `final_report` must ask for the decision and include enough context for a continuation issue/run.

Later, Simultica can add explicit continuation semantics:

- keep `workflow_run` in a paused state; or
- create a new continuation run linked to the previous run.

Until that exists, workflows must not pretend to resume automatically after human approval.

## Nova Platform E2E Sample

The `nova-platform-e2e` sample workflow should use this state machine:

```text
START
  -> triage_case
  -> author_case
  -> stabilize_run
  -> review_result
       -> author_case     when workflow_status == fixable_auto and next_target == author_case and revisionCount < 3
       -> stabilize_run   when workflow_status == fixable_auto and next_target == stabilize_run and revisionCount < 3
       -> final_report    otherwise
END
```

Budget exhaustion behavior:

- after 3 auto-fix review cycles, stop in `final_report`;
- report that user approval is required to continue a fourth round;
- do not continue burning runtime automatically.

## Implementation Phases

1. Agent-runtime validation and progress:
   - validate condition field references;
   - validate loop safety;
   - record conditional route decisions into node progress.

2. Simultica skill save validation:
   - when `workflow.yaml` is upserted, parse and validate it;
   - store warnings/errors in response or `skill.config.workflow_validation`;
   - keep `has_workflow` semantics.

3. Simultica issue runtime UI:
   - display route decisions from `nodes_state`;
   - display revision count and blocker state;
   - surface human-gate continuation copy.

4. Sample workflow migration:
   - update `nova-platform-e2e/workflow.yaml`;
   - sync it into Simultica skill file;
   - run one issue workflow smoke test when a live sidecar is available.
