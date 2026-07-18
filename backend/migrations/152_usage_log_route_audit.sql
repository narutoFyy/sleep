ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS route_mode varchar(16) NOT NULL DEFAULT 'primary',
    ADD COLUMN IF NOT EXISTS route_mapping_rule varchar(300),
    ADD COLUMN IF NOT EXISTS route_attempt_count integer NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS route_failures jsonb NOT NULL DEFAULT '[]'::jsonb;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'usage_logs_route_mode_check'
          AND conrelid = 'usage_logs'::regclass
    ) THEN
        ALTER TABLE usage_logs
            ADD CONSTRAINT usage_logs_route_mode_check
            CHECK (route_mode IN ('primary', 'standby'));
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'usage_logs_route_attempt_count_check'
          AND conrelid = 'usage_logs'::regclass
    ) THEN
        ALTER TABLE usage_logs
            ADD CONSTRAINT usage_logs_route_attempt_count_check
            CHECK (route_attempt_count >= 1);
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'usage_logs_route_failures_array_check'
          AND conrelid = 'usage_logs'::regclass
    ) THEN
        ALTER TABLE usage_logs
            ADD CONSTRAINT usage_logs_route_failures_array_check
            CHECK (jsonb_typeof(route_failures) = 'array');
    END IF;
END
$$;
