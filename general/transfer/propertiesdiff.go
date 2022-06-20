package transfer

import (
	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"time"
)

const waitTimeBetweenPropertiesStatusSeconds = 15
const propertiesPhaseDisabled = true

type propertiesDiffPhase struct {
	repoKey                   string
	checkExistenceInFilestore bool
	startTime                 time.Time
	srcUpService              *srcUserPluginService
	srcRtDetails              *coreConfig.ServerDetails
	targetRtDetails           *coreConfig.ServerDetails
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

func (p propertiesDiffPhase) shouldCheckExistenceInFilestore(shouldCheck bool) {
	p.checkExistenceInFilestore = shouldCheck
}

func (p propertiesDiffPhase) shouldSkipPhase(string) (bool, error) {
	return propertiesPhaseDisabled, nil
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
	diffStart, diffEnd, err := getDiffHandlingRange(p.repoKey)
	if err != nil {
		return err
	}

	requestBody := HandlePropertiesDiff{
		TargetAuth:        createTargetAuth(p.targetRtDetails),
		RepoKey:           p.repoKey,
		StartMilliseconds: convertTimeToEpochMilliseconds(diffStart),
		EndMilliseconds:   convertTimeToEpochMilliseconds(diffEnd),
	}

	nodes, err := getNodesList()
	if err != nil {
		return err
	}

	generalStatus := propsHandlingStatus{
		nodesStatus: make([]nodeStatus, len(nodes)),
	}

propertiesHandling:
	for {
		time.Sleep(waitTimeBetweenPropertiesStatusSeconds * time.Second)

		// Send and handle.
		remoteNodeStatus, err := p.srcUpService.handlePropertiesDiff(requestBody)
		if err != nil {
			// TODO handle error.
			return err
		}

		switch remoteNodeStatus.Status {
		case InProgress:
			err = generalStatus.handleInProgressStatus(remoteNodeStatus)
			if err != nil {
				// TODO handle error.
				return err
			}
		case Done:
			err = generalStatus.handleDoneStatus(remoteNodeStatus)
			if err != nil {
				// TODO handle error.
				return err
			}
		default:
			// TODO log error of unknown state.
		}

		for _, node := range generalStatus.nodesStatus {
			if !node.isDone {
				continue propertiesHandling
			}
		}
		notifyPropertiesProgressDone()
		return nil
	}
}

type propsHandlingStatus struct {
	nodesStatus         []nodeStatus
	totalPropsToDeliver int64
	// TODO is needed:
	totalPropsDelivered int64
}
type nodeStatus struct {
	nodeId              string
	propertiesDelivered int64
	propertiesTotal     int64
	isDone              bool
}

func (phs propsHandlingStatus) getNodeStatus(nodeId string) *nodeStatus {
	for i := range phs.nodesStatus {
		if nodeId == phs.nodesStatus[i].nodeId {
			return &phs.nodesStatus[i]
		}
	}
	// TODO handle
	return nil
}

func (phs propsHandlingStatus) handleInProgressStatus(remoteNodeStatus *HandlePropertiesDiffResponse) error {
	localNodeStatus := phs.getNodeStatus(remoteNodeStatus.NodeId)
	return phs.updateTotalAndDelivered(localNodeStatus, remoteNodeStatus)
}

func (phs propsHandlingStatus) updateTotalAndDelivered(localNodeStatus *nodeStatus, remoteNodeStatus *HandlePropertiesDiffResponse) error {
	remoteTotal, err := remoteNodeStatus.PropertiesTotal.Int64()
	if err != nil {
		// TODO handle error.
		return err
	}
	// Total has changed, update it.
	if remoteTotal != localNodeStatus.propertiesTotal {
		phs.totalPropsToDeliver += remoteTotal - localNodeStatus.propertiesTotal
		localNodeStatus.propertiesTotal = remoteTotal
		updatePropertiesProgressTotal(phs.totalPropsToDeliver)
	}

	// Delivered has changed, update it.
	delivered, err := remoteNodeStatus.PropertiesDelivered.Int64()
	if err != nil {
		// TODO handle error.
		return err
	}
	newDeliveries := delivered - localNodeStatus.propertiesDelivered
	incrementPropertiesProgress(newDeliveries)
	phs.totalPropsDelivered += newDeliveries
	localNodeStatus.propertiesDelivered = delivered
	return nil
}

func (phs propsHandlingStatus) handleDoneStatus(remoteNodeStatus *HandlePropertiesDiffResponse) error {
	localNodeStatus := phs.getNodeStatus(remoteNodeStatus.NodeId)

	// Already handled reaching done.
	if localNodeStatus.isDone {
		return nil
	}

	localNodeStatus.isDone = true
	// TODO write errors to file (probably log, not consumable).
	return phs.updateTotalAndDelivered(localNodeStatus, remoteNodeStatus)
}

func updatePropertiesProgressTotal(newTotal int64) {
	// TODO implement
}

func incrementPropertiesProgress(incr int64) {
	// TODO implement
}

func notifyPropertiesProgressDone() {
	// TODO implement
}
