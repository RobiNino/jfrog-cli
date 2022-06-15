package upload

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	coreCommonCommands "github.com/jfrog/jfrog-cli-core/v2/common/commands"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/urfave/cli"
	"io/ioutil"
	"time"
)

func RunAqlCheck(c *cli.Context) (err error) {
	sourceRtDetails, err := coreCommonCommands.GetConfig(serverID, false) //c.Args().Get(0)
	if err != nil {
		return err
	}
	serverDetails = sourceRtDetails

	startTime = time.Now()

	err = getDiffsAql("generic-robi", "*")

	return err
}

func getDiffsAql(repoKey, relativePath string) (err error) {
	query := generateAqlQuery(repoKey, relativePath)
	serviceManager, err := utils.CreateServiceManager(serverDetails, -1, 0, false)
	if err != nil {
		return err
	}
	reader, err := serviceManager.Aql(query)
	if err != nil {
		return err
	}
	defer func() {
		if reader != nil {
			e := reader.Close()
			if err == nil {
				err = errorutils.CheckError(e)
			}
		}
	}()

	log.Info("time elapsed:", time.Since(startTime))

	respBody, err := ioutil.ReadAll(reader)
	if err != nil {
		return errorutils.CheckError(err)
	}
	resString := clientutils.IndentJson(respBody)
	log.Output(resString)
	return nil

	// result = &artifactoryUtils.AqlSearchResult{}
	// err = json.Unmarshal(respBody, result)
	// return result, errorutils.CheckError(err)
}

func generateAqlQuery(repoKey, relativePath string) string {
	return fmt.Sprintf(`items.find({"type":"any","modified":{"$gt":"2022-05-29T12:50:00.00+00:00"},"$or":[{"$and":[{"repo":"%s","path":{"$match":"%s"},"name":{"$match":"*"}}]}]}).include("repo","path","name","created","modified","updated","created_by","modified_by","type","property","stat")`, repoKey, relativePath)
}
