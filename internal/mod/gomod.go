package mod

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nix-community/go-nix/pkg/nar"
	"github.com/sourcegraph/conc/pool"
	"golang.org/x/mod/modfile"
)

//nolint:tagliatelle
type goModuleDownload struct {
	Path    string `json:"Path"`
	Version string `json:"Version"`
	Dir     string `json:"Dir"`
	GoMod   string `json:"GoMod"`
}

type GoModFile struct {
	dir     string
	content []byte
	modfile *modfile.File
	hash    string
}

func ParseGoModFile(path string) (*GoModFile, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read go.mod: %w", err)
	}

	mf, err := modfile.Parse(path, content, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse go.mod: %w", err)
	}

	h := sha256.Sum256(content)
	hash := "sha256-" + base64.StdEncoding.EncodeToString(h[:])

	return &GoModFile{
		dir:     filepath.Dir(path),
		content: content,
		modfile: mf,
		hash:    hash,
	}, nil
}

func (f *GoModFile) Dir() string {
	return f.dir
}

func (f *GoModFile) Hash() string {
	return f.hash
}

func (f *GoModFile) ModulePath() string {
	return f.modfile.Module.Mod.Path
}

func (f *GoModFile) HasDependencies() bool {
	return len(f.modfile.Require) > 0
}

func (f *GoModFile) Replacements() map[string]string {
	replacements := make(map[string]string)
	for _, repl := range f.modfile.Replace {
		replacements[repl.New.Path] = repl.Old.Path
	}
	return replacements
}

func (f *GoModFile) Dependencies() ([]GoModule, error) {
	pkgsByMod, err := f.packagesByModule()
	if err != nil {
		return nil, err
	}

	downloads, err := f.downloadModules()
	if err != nil {
		return nil, err
	}

	return f.resolveModules(downloads, pkgsByMod)
}

func (f *GoModFile) packagesByModule() (map[string][]string, error) {
	cmd := []string{
		"go",
		"list",
		"-deps",
		"-f",
		fmt.Sprintf(`'{{if not .Standard}}{{if .Module}}{{if ne .Module.Path "%s"}}{{.Module.Path}}{{"\t"}}{{.ImportPath}}{{end}}{{end}}{{end}}'`, f.ModulePath()),
		"./...",
	}

	out, err := exec(cmd, f.dir)
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

func (f *GoModFile) downloadModules() ([]goModuleDownload, error) {
	cmd := []string{
		"go",
		"mod",
		"download",
		"-json",
	}

	out, err := exec(cmd, f.dir)
	if err != nil {
		return nil, err
	}

	var downloads []goModuleDownload
	dec := json.NewDecoder(strings.NewReader(out))
	for {
		var meta goModuleDownload
		if err := dec.Decode(&meta); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		downloads = append(downloads, meta)
	}

	return downloads, nil
}

func (f *GoModFile) resolveModules(downloads []goModuleDownload, pkgsByMod map[string][]string) ([]GoModule, error) {
	replacements := f.Replacements()

	p := pool.NewWithResults[GoModule]().WithErrors().WithMaxGoroutines(8)

	for _, meta := range downloads {
		p.Go(func() (GoModule, error) {
			h := sha256.New()
			if err := nar.DumpPathFilter(h, meta.Dir, func(path string, _ nar.NodeType) bool {
				return strings.ToLower(filepath.Base(path)) != ".ds_store"
			}); err != nil {
				return GoModule{}, err
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

			return GoModule{
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
