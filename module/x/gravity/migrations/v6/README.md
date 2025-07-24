# v6

The Gravity v6 migration involves moving the params from the legacy x/params module over to the Gravity module.

Unfortunately a natural implementation of this migration would cause an import cycle, so this v6 module contains no actual code and instead
the migration logic exists within keeper/migrations.go