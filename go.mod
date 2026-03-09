module github.com/agynio/token-counting

go 1.22

require (
	github.com/pkoukk/tiktoken-go v0.1.8
	github.com/pkoukk/tiktoken-go-loader v0.0.2
	go.uber.org/zap v1.27.1
	golang.org/x/image v0.24.0
	google.golang.org/grpc v1.67.0
	google.golang.org/protobuf v1.34.2
)

require (
	github.com/dlclark/regexp2 v1.10.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
)

replace golang.org/x/net => golang.org/x/net v0.26.0

replace golang.org/x/sys => golang.org/x/sys v0.13.0

replace golang.org/x/text => golang.org/x/text v0.14.0

replace google.golang.org/genproto => google.golang.org/genproto v0.0.0-20240123012728-ef4313101c80

replace google.golang.org/genproto/googleapis/api => google.golang.org/genproto/googleapis/api v0.0.0-20240123012728-ef4313101c80

replace google.golang.org/genproto/googleapis/rpc => google.golang.org/genproto/googleapis/rpc v0.0.0-20240123012728-ef4313101c80
