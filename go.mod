module github.com/jfrog/jfrog-cli

go 1.14

require (
	github.com/buger/jsonparser v1.1.1
	github.com/frankban/quicktest v1.13.1 // indirect
	github.com/go-git/go-git/v5 v5.4.2
	github.com/gookit/color v1.4.2
	github.com/jfrog/build-info-go v1.0.0
	github.com/jfrog/gofrog v1.1.1
	github.com/jfrog/jfrog-cli-core/v2 v2.8.2
	github.com/jfrog/jfrog-client-go v1.7.0
	github.com/jszwec/csvutil v1.4.0
	github.com/mholt/archiver v2.1.0+incompatible
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.7.0
	github.com/urfave/cli v1.22.5
	github.com/vbauerster/mpb/v7 v7.1.5
	github.com/xeipuuv/gojsonschema v1.2.0
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5
	gopkg.in/yaml.v2 v2.4.0
)

replace github.com/jfrog/jfrog-client-go => github.com/jfrog/jfrog-client-go v1.7.2-0.20220118105022-ec1281ed195e

replace github.com/jfrog/jfrog-cli-core/v2 => github.com/RobiNino/jfrog-cli-core/v2 v2.0.0-20220124170952-f9ab237adc5a

// replace github.com/jfrog/gofrog => github.com/jfrog/gofrog v1.0.7-0.20211128152632-e218c460d703

replace github.com/jfrog/build-info-go => github.com/jfrog/build-info-go v1.0.1-0.20220112152743-8c04d862d822
