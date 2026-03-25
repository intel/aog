module github.com/intel/aog/plugin/examples/baidu-plugin

go 1.25.0

toolchain go1.25.8

require (
	github.com/hashicorp/go-plugin v1.7.0
	github.com/intel/aog/plugin-sdk v0.0.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/fatih/color v1.18.0 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/hashicorp/go-hclog v1.6.3 // indirect
	github.com/hashicorp/yamux v0.1.2 // indirect
	github.com/kr/pretty v0.1.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/oklog/run v1.2.0 // indirect
	github.com/stretchr/testify v1.10.0 // indirect
	golang.org/x/net v0.49.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/text v0.33.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260202165425-ce8ad4cf556b // indirect
	google.golang.org/grpc v1.79.3 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
)

replace github.com/intel/aog => ../..

replace github.com/intel/aog/plugin-sdk => ../../plugin-sdk
