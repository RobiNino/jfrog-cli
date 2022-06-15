package transfer

import (
	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"time"
)

const waitTimeBetweenPropertiesStatusSeconds = 15

type propertiesDiffPhase struct {
	repoKey         string
	startTime       time.Time
	srcUpService    *srcUserPluginService
	srcRtDetails    *coreConfig.ServerDetails
	targetRtDetails *coreConfig.ServerDetails
}

func (p propertiesDiffPhase) getPhaseName() string {
	return "Properties Diff Handling Phase"
}

func (p propertiesDiffPhase) getPhaseNumber() int {
	return 3
}

func (p propertiesDiffPhase) phaseStarted() error {
	p.startTime = time.Now()
	// TODO notify progress
	return setPropsDiffHandlingStarted(p.repoKey, p.startTime)
}

func (p propertiesDiffPhase) phaseDone() error {
	// TODO notify progress
	return setPropsDiffHandlingCompleted(p.repoKey)
}

func (p propertiesDiffPhase) setRepoKey(repoKey string) {
	p.repoKey = repoKey
}

func (p propertiesDiffPhase) setSrcUserPluginService(service *srcUserPluginService) {
	p.srcUpService = service
}

func (p propertiesDiffPhase) setSourceDetails(details *coreConfig.ServerDetails) {
	p.srcRtDetails = details
}

func (p propertiesDiffPhase) setTargetDetails(details *coreConfig.ServerDetails) {
	p.targetRtDetails = details
}

func (p propertiesDiffPhase) run() error {
	// todo add check if previous diff failed.
	// todo get last migration / diff completion time
	lastCompletionTime, err := getRepoMigratedCompletionTime(p.repoKey)
	if err != nil {
		return err
	}
	filesPhaseCompletion := time.Time{}

	requestBody := HandlePropertiesDiff{
		TargetAuth:        createTargetAuth(p.targetRtDetails),
		RepoKey:           p.repoKey,
		StartMilliseconds: convertTimeToEpochMilliseconds(lastCompletionTime),
		EndMilliseconds:   convertTimeToEpochMilliseconds(filesPhaseCompletion),
	}

	go func() {
		for {
			time.Sleep(waitTimeBetweenPropertiesStatusSeconds * time.Second)

			// Send and handle.
			handlingStatus, err := p.srcUpService.handlePropertiesDiff(requestBody)
			if err != nil {
				// TODO handle error.
				return
			}

			switch handlingStatus.Status {
			case InProcess:
				// TODO increment progress.
			case Done:
				// TODO increment progress.

				// TODO write failure to file.

			default:
				// todo ?
			}
		}
	}()

	return nil
}
