ALTER TABLE account_groups
    ADD COLUMN IF NOT EXISTS role varchar(20) NOT NULL DEFAULT 'primary',
    ADD COLUMN IF NOT EXISTS enabled boolean NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS model_mapping jsonb NOT NULL DEFAULT '{}'::jsonb;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'account_groups_role_check'
          AND conrelid = 'account_groups'::regclass
    ) THEN
        ALTER TABLE account_groups
            ADD CONSTRAINT account_groups_role_check
            CHECK (role IN ('primary', 'standby'));
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'account_groups_model_mapping_object_check'
          AND conrelid = 'account_groups'::regclass
    ) THEN
        ALTER TABLE account_groups
            ADD CONSTRAINT account_groups_model_mapping_object_check
            CHECK (jsonb_typeof(model_mapping) = 'object');
    END IF;
END
$$;
