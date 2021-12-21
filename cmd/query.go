package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"golang.org/x/mod/modfile"
)

const queryDescription = `Use this command to query the contents of a package`
const elasticIntegrations = "github.com/elastic/integrations"
const packagesDir = "packages"

func setupQueryCommand() *cobraext.Command {

	queryManifestCommand := &cobra.Command{
		Use:   "manifest",
		Short: "Query packages based on manifest",
		RunE:  queryManifest,
	}
	queryManifestCommand.Flags().StringP(cobraext.ManifestKeyFlagName, "", "", cobraext.ManifestKeyFlagDescription)
	queryManifestCommand.Flags().StringSliceP(cobraext.ManifestValueFlagName, "", nil, cobraext.ManifestValueFlagDescription)
	queryManifestCommand.MarkFlagRequired(cobraext.ManifestKeyFlagName)
	queryManifestCommand.MarkFlagRequired(cobraext.ManifestValueFlagName)

	queryCmd := &cobra.Command{
		Use:   "query",
		Short: "Query packages",
		Long:  queryDescription,
	}
	queryCmd.AddCommand(queryManifestCommand)
	return cobraext.NewCommand(queryCmd, cobraext.ContextPackage)
}

func queryManifest(cmd *cobra.Command, args []string) error {
	err := queryCheck()
	if err != nil {
		return err
	}
	key, _ := cmd.Flags().GetString(cobraext.ManifestKeyFlagName)
	values, err := cmd.Flags().GetStringSlice(cobraext.ManifestValueFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.ManifestValueFlagName)
	}
	common.TrimStringSlice(values)

	ppath := filepath.Join(".", packagesDir)
	pkgs, err := ioutil.ReadDir(ppath)
	if err != nil {
		return errors.Wrap(err, "cannot find package directories")
	}
	var skipped []string
	var found []string
	for _, pkg := range pkgs {
		if pkg.IsDir() {
			manifestFile := filepath.Join(ppath, pkg.Name(), packages.PackageManifestFile)
			cfg, err := yaml.NewConfigWithFile(manifestFile, ucfg.PathSep("."))
			if err != nil {
				skipped = append(skipped, fmt.Sprintf("%s: %v", pkg.Name(), err))
				continue
			}
			if queryPackageManifest(key, values, cfg) {
				found = append(found, pkg.Name())
			}
		}
	}
	if len(skipped) > 0 {
		cmd.Println("Skipped packages:")
		for _, v := range skipped {
			cmd.Println(" ", v)
		}
	}
	if len(found) == 0 {
		cmd.Println("key with value not found in any packages")
		return nil
	}
	if len(found) > 0 {
		cmd.Println("Packages:")
		for _, v := range found {
			cmd.Println(" ", v)
		}
	}
	return nil
}

func queryPackageManifest(key string, values []string, cfg *ucfg.Config) bool {
	var opts []ucfg.Option
	opts = append(opts, ucfg.PathSep("."))
	flattenedKeys := cfg.FlattenedKeys(opts...)
	for _, f := range flattenedKeys {
		if key == f {
			v, err := cfg.String(f, 0, opts...)
			if err == nil && v == values[0] {
				return true
			}
			return false
		}
	}
	return false
}

func queryCheck() error {
	gomodfile, err := os.Open("./go.mod")
	if err != nil {
		return errors.Wrap(err, "query must be executed from the integrations project root directory")
	}
	b, err := ioutil.ReadAll(gomodfile)
	if err != nil {
		return err
	}
	modulePath := modfile.ModulePath(b)
	if modulePath != elasticIntegrations {
		return errors.New("query must be executed from the integrations project root directory")
	}
	return nil
}
