CREATE TABLE IF NOT EXISTS excuses (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    category   TEXT    NOT NULL,
    body       TEXT    NOT NULL,
    times_used INTEGER NOT NULL DEFAULT 0
);
