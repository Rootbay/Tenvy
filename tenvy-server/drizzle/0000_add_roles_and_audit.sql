ALTER TABLE `user` ADD COLUMN `role` text DEFAULT 'operator' NOT NULL;

CREATE TABLE `audit_event` (
        `id` integer PRIMARY KEY AUTOINCREMENT NOT NULL,
        `command_id` text NOT NULL,
        `agent_id` text NOT NULL,
        `operator_id` text,
        `command_name` text NOT NULL,
        `payload_hash` text NOT NULL,
        `queued_at` integer NOT NULL,
        `executed_at` integer,
        `result` text,
        FOREIGN KEY (`operator_id`) REFERENCES `user`(`id`) ON DELETE set null
);

CREATE UNIQUE INDEX `audit_event_command_idx` ON `audit_event` (`command_id`);
CREATE INDEX `audit_event_agent_idx` ON `audit_event` (`agent_id`);
