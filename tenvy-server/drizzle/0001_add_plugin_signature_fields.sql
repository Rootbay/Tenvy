-- Align existing plugin tables with signature metadata expected by the application.
ALTER TABLE plugin
	ADD COLUMN signature_status text NOT NULL DEFAULT 'unsigned';
ALTER TABLE plugin
	ADD COLUMN signature_trusted integer NOT NULL DEFAULT 0;
ALTER TABLE plugin
	ADD COLUMN signature_type text NOT NULL DEFAULT 'none';
ALTER TABLE plugin
	ADD COLUMN signature_hash text;
ALTER TABLE plugin
	ADD COLUMN signature_signer text;
ALTER TABLE plugin
	ADD COLUMN signature_public_key text;
ALTER TABLE plugin
	ADD COLUMN signature_checked_at integer;
ALTER TABLE plugin
	ADD COLUMN signature_signed_at integer;
ALTER TABLE plugin
	ADD COLUMN signature_error text;
ALTER TABLE plugin
	ADD COLUMN signature_error_code text;
ALTER TABLE plugin
	ADD COLUMN signature_chain text;

ALTER TABLE plugin_marketplace_listing
	ADD COLUMN signature_type text NOT NULL DEFAULT 'none';
ALTER TABLE plugin_marketplace_listing
	ADD COLUMN signature_hash text NOT NULL DEFAULT '';
ALTER TABLE plugin_marketplace_listing
	ADD COLUMN signature_public_key text;
ALTER TABLE plugin_marketplace_listing
	ADD COLUMN signature text NOT NULL DEFAULT '';
ALTER TABLE plugin_marketplace_listing
	ADD COLUMN signed_at integer;
ALTER TABLE plugin_marketplace_listing
	ADD COLUMN signature_status text NOT NULL DEFAULT 'unsigned';
ALTER TABLE plugin_marketplace_listing
	ADD COLUMN signature_trusted integer NOT NULL DEFAULT 0;
ALTER TABLE plugin_marketplace_listing
	ADD COLUMN signature_signer text;
ALTER TABLE plugin_marketplace_listing
	ADD COLUMN signature_checked_at integer;
ALTER TABLE plugin_marketplace_listing
	ADD COLUMN signature_error text;
ALTER TABLE plugin_marketplace_listing
	ADD COLUMN signature_error_code text;
ALTER TABLE plugin_marketplace_listing
	ADD COLUMN signature_chain text;
