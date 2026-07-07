-- name: UpsertWorkflowDefinitionDraft :one
INSERT INTO workflow_definition (
    workspace_id, case_id, draft_json, source_templates, updated_by
) VALUES (
    $1, $2, $3, sqlc.arg('source_templates'), sqlc.narg('updated_by')
)
ON CONFLICT (case_id) DO UPDATE SET
    draft_json = EXCLUDED.draft_json,
    source_templates = EXCLUDED.source_templates,
    updated_by = EXCLUDED.updated_by,
    updated_at = now()
RETURNING *;

-- name: GetWorkflowDefinitionByCase :one
SELECT * FROM workflow_definition WHERE case_id = $1;

-- name: CreateWorkflowDefinitionVersion :one
INSERT INTO workflow_definition_version (
    workspace_id, case_id, definition_id, version, snapshot_json, source_skills, validation_report, confirmed_by
) VALUES (
    $1, $2, $3,
    COALESCE((SELECT MAX(version) + 1 FROM workflow_definition_version WHERE definition_id = $3), 1),
    $4, sqlc.arg('source_skills'), sqlc.arg('validation_report'), sqlc.narg('confirmed_by')
)
RETURNING *;

-- name: GetWorkflowDefinitionVersion :one
SELECT * FROM workflow_definition_version WHERE id = $1;

-- name: ListWorkflowDefinitionVersions :many
SELECT * FROM workflow_definition_version
WHERE case_id = $1
ORDER BY version DESC;
