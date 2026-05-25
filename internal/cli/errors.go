package cli

import "errors"

// ErrUsage signals a CLI usage problem (bad args, unknown subcommand,
// missing required value). The composition root maps it to exit code 2,
// distinct from operational failures which exit 1.
var ErrUsage = errors.New("usage")
