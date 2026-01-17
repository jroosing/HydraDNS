# Database Migrations

HydraDNS uses [golang-migrate](https://github.com/golang-migrate/migrate) for database schema migrations.

## Overview

Migrations are automatically applied when the application starts. The migration files are embedded in the binary and executed against the SQLite database.

## Migration Files

Migration files are located in `internal/database/migrations/` and follow the naming convention:
```
{version}_{name}.{up|down}.sql
```

For example:
- `000001_init_schema.up.sql` - Creates initial schema
- `000001_init_schema.down.sql` - Rolls back initial schema

## Creating New Migrations

To create a new migration:

1. Determine the next version number (e.g., if `000001` exists, use `000002`)
2. Create two files:
   - `{version}_{description}.up.sql` - Contains the migration
   - `{version}_{description}.down.sql` - Contains the rollback

Example:
```bash
# Create new migration files
touch internal/database/migrations/000002_add_cache_table.up.sql
touch internal/database/migrations/000002_add_cache_table.down.sql
```

### Up Migration (`*.up.sql`)
Contains SQL statements to apply the migration:
```sql
CREATE TABLE IF NOT EXISTS cache (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    expires_at TIMESTAMP NOT NULL
);
```

### Down Migration (`*.down.sql`)
Contains SQL statements to rollback the migration:
```sql
DROP TABLE IF EXISTS cache;
```

## Migration Process

Migrations are automatically applied during application startup in `internal/database/db.go`:

1. The application connects to SQLite
2. The embedded migration files are loaded
3. golang-migrate checks the current schema version
4. Any pending migrations are applied in order
5. The schema version is updated

## Manual Migration Management

While migrations are automatic, you can use the `migrate` CLI tool for manual operations:

```bash
# Install migrate CLI
go install -tags 'sqlite' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Check current version
migrate -path internal/database/migrations -database "sqlite://hydradns.db" version

# Apply all pending migrations
migrate -path internal/database/migrations -database "sqlite://hydradns.db" up

# Rollback one migration
migrate -path internal/database/migrations -database "sqlite://hydradns.db" down 1

# Go to specific version
migrate -path internal/database/migrations -database "sqlite://hydradns.db" goto 2
```

## Best Practices

1. **Never modify existing migrations** - Once a migration is committed and released, create a new migration instead
2. **Always test rollbacks** - Ensure down migrations work correctly
3. **Use IF NOT EXISTS** - Makes migrations idempotent for safety
4. **Keep migrations small** - One logical change per migration
5. **Add indexes in separate migrations** - Easier to rollback if needed
6. **Test with fresh database** - Verify migrations work from scratch

## Troubleshooting

### Migration fails with "no change"
This is normal - it means all migrations are already applied.

### Migration fails with "Dirty database version"
This happens if a migration failed partway through. Fix manually:
```bash
migrate -path internal/database/migrations -database "sqlite://hydradns.db" force {version}
```

### Start fresh
Delete the database file and restart the application:
```bash
rm hydradns.db*
./hydradns
```

## Schema Versioning

The database maintains two version tracking systems:

1. **Migration Version** - Tracked by golang-migrate in `schema_migrations` table
2. **Config Version** - Application-level version in `config_version` table for sync purposes

The config version automatically increments on data changes via SQLite triggers, enabling efficient primary/secondary synchronization.
