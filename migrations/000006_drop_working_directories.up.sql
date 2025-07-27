-- Drop the working_directories table as it's no longer needed
DROP INDEX IF EXISTS idx_working_directories_channel_id;
DROP TABLE IF EXISTS working_directories;