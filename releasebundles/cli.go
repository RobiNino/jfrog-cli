package releasebundles

import (
	"github.com/jfrog/jfrog-cli-core/v2/common/commands"
	coreCommon "github.com/jfrog/jfrog-cli-core/v2/docs/common"
	"github.com/jfrog/jfrog-cli-core/v2/releasebundles"
	"github.com/jfrog/jfrog-cli/docs/common"
	rbCreate "github.com/jfrog/jfrog-cli/docs/releasebundles/create"
	rbPromote "github.com/jfrog/jfrog-cli/docs/releasebundles/promote"
	"github.com/jfrog/jfrog-cli/utils/cliutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/urfave/cli"
)

const rbCategory = "Release Bundles"

func GetCommands() []cli.Command {
	return cliutils.GetSortedCommands(cli.CommandsByName{
		{
			Name:         "release-bundle-create",
			Aliases:      []string{"rbc"},
			Flags:        cliutils.GetCommandFlags(cliutils.ReleaseBundleCreate),
			Usage:        rbCreate.GetDescription(),
			HelpName:     coreCommon.CreateUsage("release-bundle-create", rbCreate.GetDescription(), rbCreate.Usage),
			UsageText:    rbCreate.GetArguments(),
			ArgsUsage:    common.CreateEnvVars(),
			BashComplete: coreCommon.CreateBashCompletionFunc(),
			Category:     rbCategory,
			Action: func(c *cli.Context) error {
				return create(c)
			},
		},
		{
			Name:         "release-bundle-promote",
			Aliases:      []string{"rbp"},
			Flags:        cliutils.GetCommandFlags(cliutils.ReleaseBundlePromote),
			Usage:        rbPromote.GetDescription(),
			HelpName:     coreCommon.CreateUsage("release-bundle-promote", rbPromote.GetDescription(), rbPromote.Usage),
			UsageText:    rbPromote.GetArguments(),
			ArgsUsage:    common.CreateEnvVars(),
			BashComplete: coreCommon.CreateBashCompletionFunc(),
			Category:     rbCategory,
			Action: func(c *cli.Context) error {
				return promote(c)
			},
		},
	})
}

func validateCreateReleaseBundleContext(c *cli.Context) error {
	if show, err := cliutils.ShowCmdHelpIfNeeded(c, c.Args()); show || err != nil {
		return err
	}

	if c.NArg() != 3 {
		return cliutils.WrongNumberOfArgumentsHandler(c)
	}

	buildSourceProvided := c.String(cliutils.BuildSource) != ""
	rbSourceProvided := c.String(cliutils.ReleaseBundlesSource) != ""
	if (buildSourceProvided && rbSourceProvided) ||
		!(buildSourceProvided || rbSourceProvided) {
		return errorutils.CheckErrorf("exactly one of the following options must be supplied: --%s or --%s", cliutils.BuildSource, cliutils.ReleaseBundlesSource)
	}

	if !buildSourceProvided && c.String(cliutils.BuildProject) != "" {
		return errorutils.CheckErrorf("the --%s option is mandatory when providing --%s", cliutils.BuildSource, cliutils.BuildProject)
	}
	return nil
}

func create(c *cli.Context) (err error) {
	if err = validateCreateReleaseBundleContext(c); err != nil {
		return err
	}

	rtDetails, err := cliutils.CreateArtifactoryDetailsByFlags(c)
	if err != nil {
		return
	}

	createCmd := releasebundles.NewReleaseBundleCreate().SetServerDetails(rtDetails).SetReleaseBundleName(c.Args().Get(0)).SetReleaseBundleVersion(c.Args().Get(1)).
		SetCryptoKeyName(c.Args().Get(2)).SetAsync(c.BoolT(cliutils.Async)).SetReleaseBundleProject(c.String("project")).
		SetBuildSource(c.String(cliutils.BuildSource)).SetBuildProject(c.String(cliutils.BuildProject)).SetSourceReleaseBundlesPath(c.String(cliutils.ReleaseBundlesSource))
	return commands.Exec(createCmd)
}

func promote(c *cli.Context) (err error) {
	if show, err := cliutils.ShowCmdHelpIfNeeded(c, c.Args()); show || err != nil {
		return err
	}

	if c.NArg() != 3 {
		return cliutils.WrongNumberOfArgumentsHandler(c)
	}

	rtDetails, err := cliutils.CreateArtifactoryDetailsByFlags(c)
	if err != nil {
		return
	}

	createCmd := releasebundles.NewReleaseBundlePromote().SetServerDetails(rtDetails).SetReleaseBundleName(c.Args().Get(0)).SetReleaseBundleVersion(c.Args().Get(1)).
		SetCryptoKeyName(c.Args().Get(2)).SetAsync(c.BoolT(cliutils.Async)).SetReleaseBundleProject(c.String("project")).
		SetEnvironment(c.String(cliutils.Environment)).SetOverwrite(c.Bool(cliutils.Overwrite))
	return commands.Exec(createCmd)
}
