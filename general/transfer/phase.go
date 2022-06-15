package transfer

import coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"

const numberOfPhases = 3

type transferPhase interface {
	run() error
	phaseStarted() error
	phaseDone() error
	setRepoKey(string)
	setSrcUserPluginService(*srcUserPluginService)
	setSourceDetails(*coreConfig.ServerDetails)
	setTargetDetails(*coreConfig.ServerDetails)

	// todo not used:
	getPhaseName() string
	getPhaseNumber() int

	// todo add when progress
	//SetProgress(ioUtils.ProgressMgr)
	//IncrementProgress()
}
