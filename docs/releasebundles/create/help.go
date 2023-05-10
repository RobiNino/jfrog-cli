package create

var Usage = []string{"rbc [command options] <release bundle name> <release bundle version> <crypto key name>"}

func GetDescription() string {
	return "Create release bundle from build or from aggregated release bundles"
}

func GetArguments() string {
	return `	release bundle name
		Name of the newly created Release Bundle.

	release bundle version
		Version of the newly created Release Bundle.

	crypto key name
		The GPG/RSA key-pair name given in Artifactory.`
}
