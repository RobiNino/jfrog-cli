package main

import (
	"encoding/json"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli/inttestutils"
	"github.com/jfrog/jfrog-cli/utils/tests"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services/releasebundles"
	"github.com/jfrog/jfrog-client-go/config"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/stretchr/testify/assert"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

const (
	rbMinVersion   = "7.45.0"
	gpgKeyPairName = "rb-tests-key-pair"
	rbTestdataPath = "releasebundles"
)

func TestReleaseBundles(t *testing.T) {
	initReleaseBundlesTest(t)
	defer cleanRBTests(t)
	rtManager := getRtServiceManager(t)

	// Upload builds to create release bundles from.
	number1, number2, number3 := "111", "222", "333"
	uploadBuild(t, tests.UploadDevSpecA, tests.RbBuildName1, number1)
	uploadBuild(t, tests.UploadDevSpecB, tests.RbBuildName2, number2)
	defer inttestutils.DeleteBuild(serverDetails.ArtifactoryUrl, tests.RtBuildName1, artHttpDetails)
	defer inttestutils.DeleteBuild(serverDetails.ArtifactoryUrl, tests.RtBuildName2, artHttpDetails)

	// Create release bundles from builds.
	assert.NoError(t, platformCli.Exec("rbc", tests.RbRbName1, number1, gpgKeyPairName, "--build-source="+tests.RbBuildName1, "--build-project=default", "--async=false"))
	assert.NoError(t, platformCli.Exec("rbc", tests.RbRbName2, number2, gpgKeyPairName, "--build-source="+tests.RbBuildName2+"/"+number2, "--project=default"))
	assertStatusSubmitted(t, rtManager, tests.RbRbName2, number2, "")
	defer deleteReleaseBundle(t, rtManager, tests.RbRbName1, number1)
	defer deleteReleaseBundle(t, rtManager, tests.RbRbName2, number2)

	// Create a combined release bundle from the two previous release bundle.
	rbSourceFile, err := getRbSourceFile()
	assert.NoError(t, err)
	assert.NoError(t, platformCli.Exec("rbc", tests.RbRbName3, number3, gpgKeyPairName, "--rb-source="+rbSourceFile, "--async=false"))
	defer deleteReleaseBundle(t, rtManager, tests.RbRbName3, number3)

	// Promote the last release bundle.
	promoteRb(t, rtManager, number3)

	// Verify the artifacts of both the initial release bundles made it to the prod repo.
	searchSpec, err := tests.CreateSpec(tests.SearchAllProdRepo)
	assert.NoError(t, err)
	inttestutils.VerifyExistInArtifactory(tests.GetExpectedReleaseBundlesArtifacts(), searchSpec, serverDetails, t)
}

func promoteRb(t *testing.T, rtManager artifactory.ArtifactoryServicesManager, rbVersion string) {
	output := platformCli.RunCliCmdWithOutput(t, "rbp", tests.RbRbName3, rbVersion, gpgKeyPairName, "--env=PROD", "--overwrite=true", "--async=true", "--project=default")
	var promotionResp releasebundles.RbPromotionResp
	if !assert.NoError(t, json.Unmarshal([]byte(output), &promotionResp)) {
		return
	}
	assertStatusSubmitted(t, rtManager, tests.RbRbName3, rbVersion, promotionResp.CreatedMillis.String())
}

func getRbSourceFile() (string, error) {
	source := filepath.Join(tests.GetTestResourcesPath(), rbTestdataPath, "rb-source.json")
	return tests.ReplaceTemplateVariables(source, "")
}

// If createdMillis is provided, assert status for promotion. If blank, assert for creation.
func assertStatusSubmitted(t *testing.T, rtManager artifactory.ArtifactoryServicesManager, rbName, rbVersion, createdMillis string) {
	resp, err := getStatus(rtManager, rbName, rbVersion, createdMillis)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, releasebundles.Submitted, resp.Status)
}

func getRtServiceManager(t *testing.T) artifactory.ArtifactoryServicesManager {
	serviceConfig, err := config.NewConfigBuilder().
		SetServiceDetails(artAuth).
		SetDryRun(false).
		Build()
	assert.NoError(t, err)
	rtManager, err := artifactory.New(serviceConfig)
	assert.NoError(t, err)
	return rtManager
}

func getStatus(rtManager artifactory.ArtifactoryServicesManager, rbName, rbVersion, createdMillis string) (releasebundles.ReleaseBundleStatusResponse, error) {
	rbDetails := releasebundles.ReleaseBundleDetails{
		ReleaseBundleName:    rbName,
		ReleaseBundleVersion: rbVersion,
	}

	if createdMillis == "" {
		return rtManager.GetReleaseBundleCreateStatus(rbDetails, "", true)
	}
	return rtManager.GetReleaseBundlePromotionStatus(rbDetails, "", createdMillis, true)
}

func deleteReleaseBundle(t *testing.T, rtManager artifactory.ArtifactoryServicesManager, rbName, rbVersion string) {
	rbDetails := releasebundles.ReleaseBundleDetails{
		ReleaseBundleName:    rbName,
		ReleaseBundleVersion: rbVersion,
	}

	assert.NoError(t, rtManager.DeleteReleaseBundle(rbDetails, releasebundles.ReleaseBundleQueryParams{Async: false}))
}

func uploadBuild(t *testing.T, specFileName, buildName, buildNumber string) {
	specFile, err := tests.CreateSpec(specFileName)
	assert.NoError(t, err)
	runRt(t, "upload", "--spec="+specFile, "--build-name="+buildName, "--build-number="+buildNumber)
	runRt(t, "build-publish", buildName, buildNumber)
}

func initReleaseBundlesTest(t *testing.T) {
	if !*tests.TestReleaseBundles {
		t.Skip("Skipping release bundle test. To run release bundle test add the '-test.rb=true' option.")
	}
	validateArtifactoryVersion(t, rbMinVersion)

	if !isRbSupported(t) {
		t.Skip("Skipping release bundle test because the functionality is not enabled on the provided JPD.")
	}
}

func isRbSupported(t *testing.T) (skip bool) {
	client, err := httpclient.ClientBuilder().Build()
	assert.NoError(t, err)

	resp, _, _, err := client.SendGet(serverDetails.ArtifactoryUrl+"api/release_bundles/records/no-existing-rb", true, artHttpDetails, "")
	if !assert.NoError(t, err) {
		return
	}
	return resp.StatusCode != http.StatusNotImplemented
}

func InitRBTests() {
	initArtifactoryCli()
	cleanUpOldBuilds()
	cleanUpOldRepositories()
	cleanUpOldUsers()
	tests.AddTimestampToGlobalVars()
	createRequiredRepos()
	sendGpgKeyPair()
}

func CleanRBTests() {
	deleteCreatedRepos()
}

func cleanRBTests(t *testing.T) {
	deleteFilesFromRepo(t, tests.RtDevRepo)
	deleteFilesFromRepo(t, tests.RtProdRepo)
	tests.CleanFileSystem()
}

func sendGpgKeyPair() {
	// Create http client
	client, err := httpclient.ClientBuilder().Build()
	coreutils.ExitOnErr(err)

	// Check if one already exists
	resp, body, _, err := client.SendGet(*tests.JfrogUrl+"artifactory/api/security/keypair/"+gpgKeyPairName, true, artHttpDetails, "")
	coreutils.ExitOnErr(err)
	if resp.StatusCode == http.StatusOK {
		return
	}
	coreutils.ExitOnErr(errorutils.CheckResponseStatusWithBody(resp, body, http.StatusNotFound))

	// Read gpg public and private keys
	keysDir := filepath.Join(tests.GetTestResourcesPath(), rbTestdataPath, "keys")
	publicKey, err := os.ReadFile(filepath.Join(keysDir, "public.txt"))
	coreutils.ExitOnErr(err)
	privateKey, err := os.ReadFile(filepath.Join(keysDir, "private.txt"))
	coreutils.ExitOnErr(err)

	// Send keys to Artifactory
	payload := KeyPairPayload{
		PairName:   gpgKeyPairName,
		PairType:   "GPG",
		Alias:      gpgKeyPairName + "-alias",
		Passphrase: "password",
		PublicKey:  string(publicKey),
		PrivateKey: string(privateKey),
	}
	content, err := json.Marshal(payload)
	coreutils.ExitOnErr(err)
	resp, body, err = client.SendPost(*tests.JfrogUrl+"artifactory/api/security/keypair", content, artHttpDetails, "")
	coreutils.ExitOnErr(err)
	coreutils.ExitOnErr(errorutils.CheckResponseStatusWithBody(resp, body, http.StatusCreated))
}

type KeyPairPayload struct {
	PairName   string `json:"pairName,omitempty"`
	PairType   string `json:"pairType,omitempty"`
	Alias      string `json:"alias,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
	PublicKey  string `json:"publicKey,omitempty"`
	PrivateKey string `json:"privateKey,omitempty"`
}
