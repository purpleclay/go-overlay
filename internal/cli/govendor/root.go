package govendor

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/nix-community/go-nix/pkg/nar"
	"github.com/sourcegraph/conc/pool"
	"github.com/spf13/cobra"
	"golang.org/x/mod/modfile"
)

const (
	vendorFile = "govendor.toml"
)

//nolint:tagliatelle
type goModuleMetadata struct {
	Path    string `json:"Path"`
	Version string `json:"Version"`
	Dir     string `json:"Dir"`
	GoMod   string `json:"GoMod"`
}

type vendorManifest struct {
	Schema int                 `toml:"schema"`
	Mod    map[string]goModule `toml:"mod"`
}

type goModule struct {
	Path         string   `toml:"-"`
	Version      string   `toml:"version"`
	Hash         string   `toml:"hash"`
	GoVersion    string   `toml:"go,omitempty"`
	Packages     []string `toml:"packages,omitempty"`
	ReplacedPath string   `toml:"replaced,omitempty"`
}

func Execute(out io.Writer) error {
	cmd := &cobra.Command{
		Use:   "go-vendor",
		Short: "Generate a vendor manifest for building Go applications with Nix",
		Long: `Generate a govendor.toml manifest containing Go module metadata for use
with go-overlay's buildGoApplication Nix function.

The manifest includes module versions, NAR hashes, Go version requirements,
and package lists. This metadata enables Nix to build Go applications using
vendored dependencies without requiring nixpkgs' patched Go toolchain.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			manifest, err := vendor()
			if err != nil {
				return err
			}

			var buf bytes.Buffer
			encoder := toml.NewEncoder(&buf)
			if err := encoder.Encode(manifest); err != nil {
				return err
			}

			if err := os.WriteFile(vendorFile, buf.Bytes(), 0o644); err != nil {
				return err
			}

			fmt.Fprintf(out, "wrote %s with %d modules\n", vendorFile, len(manifest.Mod))
			return nil
		},
	}

	return cmd.Execute()
}

func vendor() (*vendorManifest, error) {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		return nil, err
	}

	goMod, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return nil, err
	}

	replacements := make(map[string]string)
	for _, repl := range goMod.Replace {
		replacements[repl.New.Path] = repl.Old.Path
	}

	pkgsByMod, err := packagesByModule(goMod.Module.Mod.Path)
	if err != nil {
		return nil, err
	}

	downloads, err := downloadModules()
	if err != nil {
		return nil, err
	}

	goModules, err := resolveGoModules(downloads, pkgsByMod, replacements)
	if err != nil {
		return nil, err
	}

	mod := map[string]goModule{}
	for _, goModule := range goModules {
		mod[goModule.Path] = goModule
	}

	return &vendorManifest{
		Schema: 1,
		Mod:    mod,
	}, nil
}

func packagesByModule(ownModule string) (map[string][]string, error) {
	cmd := []string{
		"go",
		"list",
		"-deps",
		"-f",
		fmt.Sprintf(`'{{if not .Standard}}{{if .Module}}{{if ne .Module.Path "%s"}}{{.Module.Path}}{{"\t"}}{{.ImportPath}}{{end}}{{end}}{{end}}'`, ownModule),
		"./...",
	}

	out, err := exec(cmd)
	if err != nil {
		return nil, err
	}

	pkgsByMod := make(map[string][]string)
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		modPath, pkgPath, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		pkgsByMod[modPath] = append(pkgsByMod[modPath], pkgPath)
	}

	for modPath := range pkgsByMod {
		sort.Strings(pkgsByMod[modPath])
	}

	return pkgsByMod, nil
}

func downloadModules() ([]goModuleMetadata, error) {
	cmd := []string{
		"go",
		"mod",
		"download",
		"-json",
	}

	out, err := exec(cmd)
	if err != nil {
		return nil, err
	}

	var downloads []goModuleMetadata
	dec := json.NewDecoder(strings.NewReader(out))
	for {
		var meta goModuleMetadata
		if err := dec.Decode(&meta); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		downloads = append(downloads, meta)
	}

	return downloads, nil
}

func resolveGoModules(moduleMetadata []goModuleMetadata, pkgsByMod map[string][]string, replacements map[string]string) ([]goModule, error) {
	p := pool.NewWithResults[goModule]().WithErrors().WithMaxGoroutines(8)

	for _, meta := range moduleMetadata {
		p.Go(func() (goModule, error) {
			h := sha256.New()
			if err := nar.DumpPathFilter(h, meta.Dir, func(path string, _ nar.NodeType) bool {
				return strings.ToLower(filepath.Base(path)) != ".ds_store"
			}); err != nil {
				return goModule{}, err
			}

			digest := h.Sum(nil)
			hash := "sha256-" + base64.StdEncoding.EncodeToString(digest)

			var goVersion string
			if meta.GoMod != "" {
				if modData, err := os.ReadFile(meta.GoMod); err == nil {
					if modFile, err := modfile.Parse(meta.GoMod, modData, nil); err == nil && modFile.Go != nil {
						goVersion = modFile.Go.Version
					}
				}
			}

			path := meta.Path
			var replacedPath string
			if orig, ok := replacements[path]; ok {
				path = orig
				replacedPath = meta.Path
			}

			return goModule{
				Path:         path,
				Version:      meta.Version,
				Packages:     pkgsByMod[meta.Path],
				Hash:         hash,
				GoVersion:    goVersion,
				ReplacedPath: replacedPath,
			}, nil
		})
	}

	goModules, err := p.Wait()
	if err != nil {
		return nil, err
	}

	sort.Slice(goModules, func(i, j int) bool {
		return goModules[i].Path < goModules[j].Path
	})

	return goModules, nil
}
