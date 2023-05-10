package releasebundles

import (
	"github.com/jfrog/jfrog-cli/utils/cliutils"
	"github.com/jfrog/jfrog-cli/utils/tests"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestValidateCreateReleaseBundleContext(t *testing.T) {
	testRuns := []struct {
		name        string
		args        []string
		flags       []string
		expectError bool
	}{
		{"withoutArgs", []string{}, []string{}, true},
		{"oneArg", []string{"one"}, []string{}, true},
		{"twoArgs", []string{"one", "two"}, []string{}, true},
		{"extraArgs", []string{"one", "two", "three", "four"}, []string{}, true},
		{"bothSources", []string{"one", "two", "three"}, []string{cliutils.BuildSource + "=build/number", cliutils.ReleaseBundlesSource + "=/path/to/file"}, true},
		{"noSources", []string{"one", "two", "three"}, []string{}, true},
		{"buildProjectWithoutBuildSource", []string{"one", "two", "three"}, []string{cliutils.BuildProject + "=project"}, true},
		{"buildProjectWithRbSource", []string{"one", "two", "three"}, []string{cliutils.BuildProject + "=project", cliutils.ReleaseBundlesSource + "=/path/to/file"}, true},
		{"buildSourceWithProject", []string{"one", "two", "three"}, []string{cliutils.BuildSource + "=build/number", cliutils.BuildProject + "=project"}, false},
		{"buildSourceWithoutProject", []string{"one", "two", "three"}, []string{cliutils.BuildSource + "=build/number"}, false},
		{"rbSource", []string{"one", "two", "three"}, []string{cliutils.ReleaseBundlesSource + "=/path/to/file"}, false},
	}

	for _, test := range testRuns {
		t.Run(test.name, func(t *testing.T) {
			context, buffer := tests.CreateContext(t, test.flags, test.args)
			err := validateCreateReleaseBundleContext(context)
			if test.expectError {
				assert.Error(t, err, buffer)
			} else {
				assert.NoError(t, err, buffer)
			}
		})
	}
}
