package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/guilherme-grimm/basego/internal/oapi"
	"github.com/guilherme-grimm/basego/internal/scaffold"
)

// nameRe matches a conservative project-name shape: starts with a letter,
// then letters/digits/dash/underscore. Used for the directory name; module
// path is validated separately.
var nameRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

// supportedDrivers is the closed set of DB drivers basego knows how to
// scaffold. Memory is always included regardless of --db.
var supportedDrivers = map[string]struct{}{
	"memory":   {},
	"mongo":    {},
	"postgres": {},
}

// supportedExtensions lists extensions that can follow the project name.
// Each value is the number of positional args the extension consumes after
// its keyword.
var supportedExtensions = map[string]int{
	"file": 1,
}

// ParseCreate turns the argv slice that follows `basego create` into a
// validated CreateRequest. It does not touch the filesystem.
func ParseCreate(args []string) (*scaffold.CreateRequest, error) {
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var module, dbList string
	fs.StringVar(&module, "module", "", "Go module path (default: project name)")
	fs.StringVar(&dbList, "db", "", "comma-separated DB drivers; memory is always included")
	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("flag parse: %w", err)
	}
	rest := fs.Args()
	if len(rest) == 0 {
		return nil, fmt.Errorf("missing project name")
	}
	name := rest[0]
	if !nameRe.MatchString(name) {
		return nil, fmt.Errorf("invalid project name %q: must start with a letter and contain only letters, digits, '-' or '_'", name)
	}
	if module == "" {
		module = name
	}
	if err := validateModule(module); err != nil {
		return nil, err
	}
	drivers, err := normalizeDrivers(dbList)
	if err != nil {
		return nil, err
	}
	exts, err := parseExtensions(rest[1:])
	if err != nil {
		return nil, err
	}
	return &scaffold.CreateRequest{
		Name:       name,
		Module:     module,
		Drivers:    drivers,
		Extensions: exts,
	}, nil
}

// LoadSpec resolves the `file <spec.yaml>` extension by reading the file
// and parsing it. It mutates req in place. Returns nil if no file
// extension is present.
func LoadSpec(req *scaffold.CreateRequest) error {
	for _, e := range req.Extensions {
		if e.Name != "file" {
			continue
		}
		path := e.Args[0]
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read spec %s: %w", path, err)
		}
		spec, err := oapi.Parse(data)
		if err != nil {
			return err
		}
		req.Spec = spec
		req.SpecBytes = data
		return nil
	}
	return nil
}

func validateModule(m string) error {
	if strings.TrimSpace(m) == "" {
		return fmt.Errorf("module path is empty")
	}
	if strings.ContainsAny(m, " \t\n") {
		return fmt.Errorf("invalid module path %q: contains whitespace", m)
	}
	return nil
}

// normalizeDrivers parses the comma-separated --db value, validates each
// entry against supportedDrivers, rejects duplicates, ensures memory is
// present, and returns a sorted slice. Sorting is required by PLAN §5
// (determinism: same input, same output).
func normalizeDrivers(list string) ([]string, error) {
	seen := map[string]bool{}
	if list != "" {
		for _, raw := range strings.Split(list, ",") {
			d := strings.TrimSpace(strings.ToLower(raw))
			if d == "" {
				return nil, fmt.Errorf("empty driver in --db list")
			}
			if _, ok := supportedDrivers[d]; !ok {
				return nil, fmt.Errorf("unknown driver %q (supported: memory, mongo, postgres)", d)
			}
			if seen[d] {
				return nil, fmt.Errorf("duplicate driver %q in --db list", d)
			}
			seen[d] = true
		}
	}
	seen["memory"] = true
	out := make([]string, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	sort.Strings(out)
	return out, nil
}

// parseExtensions walks the positionals after the project name and groups
// them into recognized extensions. Each extension keyword consumes a fixed
// number of following args per supportedExtensions.
func parseExtensions(args []string) ([]scaffold.Extension, error) {
	var out []scaffold.Extension
	for i := 0; i < len(args); {
		kw := args[i]
		argc, ok := supportedExtensions[kw]
		if !ok {
			return nil, fmt.Errorf("unsupported extension %q", kw)
		}
		if i+argc >= len(args) {
			return nil, fmt.Errorf("extension %q requires %d argument(s)", kw, argc)
		}
		ext := scaffold.Extension{Name: kw, Args: append([]string(nil), args[i+1:i+1+argc]...)}
		if err := validateExtensionArgs(ext); err != nil {
			return nil, err
		}
		out = append(out, ext)
		i += 1 + argc
	}
	return out, nil
}

func validateExtensionArgs(e scaffold.Extension) error {
	switch e.Name {
	case "file":
		spec := e.Args[0]
		if !strings.HasSuffix(spec, ".yaml") && !strings.HasSuffix(spec, ".yml") {
			return fmt.Errorf("extension 'file' expects a .yaml/.yml spec, got %q", spec)
		}
	}
	return nil
}

func runCreate(args []string, stdout, stderr io.Writer) error {
	req, err := ParseCreate(args)
	if err != nil {
		fmt.Fprintf(stderr, "basego create: %s\n", err)
		fmt.Fprintln(stderr, "usage: basego create [--module=path] [--db=driver,...] <name> [extension ...]")
		return ErrUsage
	}
	if err := LoadSpec(req); err != nil {
		fmt.Fprintf(stderr, "basego create: %s\n", err)
		return err
	}
	plan, err := scaffold.Render(req)
	if err != nil {
		fmt.Fprintf(stderr, "basego create: render: %s\n", err)
		return err
	}
	if err := scaffold.Write(plan); err != nil {
		fmt.Fprintf(stderr, "basego create: %s\n", err)
		return err
	}
	fmt.Fprintf(stdout, "scaffolded %q at ./%s\n", req.Name, plan.Target)
	return nil
}
