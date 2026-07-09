-- Review Step support: allow a run to pause at a human review gate and a node
-- to sit in pending_review while it waits for an approve/reject decision.

ALTER TABLE workflow_run
  DROP CONSTRAINT IF EXISTS workflow_run_status_check;

ALTER TABLE workflow_run
  ADD CONSTRAINT workflow_run_status_check
  CHECK (status IN ('pending','planning','running','waiting_for_review','finalizing','done','failed','cancelling','cancelled'));

ALTER TABLE workflow_run_node
  DROP CONSTRAINT IF EXISTS workflow_run_node_status_check;

ALTER TABLE workflow_run_node
  ADD CONSTRAINT workflow_run_node_status_check
  CHECK (status IN ('pending','running','pending_review','blocked','succeeded','failed','cancelling','cancelled','skipped'));

ALTER TABLE workflow_run_node_event
  DROP CONSTRAINT IF EXISTS workflow_run_node_event_event_type_check;

ALTER TABLE workflow_run_node_event
  ADD CONSTRAINT workflow_run_node_event_event_type_check
  CHECK (event_type IN ('node_started','node_blocked','node_succeeded','node_failed','node_cancelled','node_skipped','node_carrier_attached','node_log_appended','node_review_requested','node_review_approved','node_review_rejected'));
