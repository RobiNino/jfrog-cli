package create

var Usage = []string{"rbp [command options] <release bundle name> <release bundle version> <crypto key name>"}

func GetDescription() string {
	return "Promote Release Bundle"
}

func GetArguments() string {
	return `	release bundle name
		Name of the Release Bundle to promote.

	release bundle version
		Version of the Release Bundle to promote.

	crypto key name
		The GPG/RSA key-pair name given in Artifactory.`
}
