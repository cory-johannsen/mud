-- character_jobs: stores all jobs a character holds (multi-job, REQ-JD-4).
-- characters.job (existing) remains as the active job ID for backward compat.
CREATE TABLE IF NOT EXISTS character_jobs (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    job_id       TEXT   NOT NULL,
    PRIMARY KEY (character_id, job_id)
);
