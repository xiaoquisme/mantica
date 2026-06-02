-- Revert: make runtime_id NOT NULL again
ALTER TABLE agent ALTER COLUMN runtime_id SET NOT NULL;
