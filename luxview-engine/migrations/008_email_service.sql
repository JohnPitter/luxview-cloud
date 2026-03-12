-- 008: Email service support — mailboxes table + plan limits
-- Phase B of email hosting feature

ALTER TABLE plans ADD COLUMN IF NOT EXISTS max_mailboxes_per_app INTEGER NOT NULL DEFAULT 0;
ALTER TABLE plans ADD COLUMN IF NOT EXISTS max_mailbox_storage VARCHAR(20) NOT NULL DEFAULT '500m';

CREATE TABLE IF NOT EXISTS mailboxes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id UUID NOT NULL REFERENCES app_services(id) ON DELETE CASCADE,
    address VARCHAR(255) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_mailboxes_service_id ON mailboxes(service_id);
