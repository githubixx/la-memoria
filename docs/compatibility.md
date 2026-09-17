# Compatibility

The importable library starts at version `0.1.0`. Before `1.0.0`, incompatible
library changes increment the MINOR version and include migration notes. At or
after `1.0.0`, incompatible changes increment MAJOR and include a migration
guide.

The HTTP form/route contract and CLI JSON Lines protocol are independently
versioned as `v1`. Removing or changing a request field, response field,
operation, or error-code meaning requires a new protocol version regardless of
the library version. PostgreSQL schema changes are independently versioned by
forward migrations; compatibility notes accompany every new migration.
