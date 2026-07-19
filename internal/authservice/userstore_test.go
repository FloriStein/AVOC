package authservice

// Compile-time check (GOSTYLE-IF-04): PostgresUserStore must keep satisfying
// UserStore now that SeedAdmin was removed from the interface.
var _ UserStore = (*PostgresUserStore)(nil)
