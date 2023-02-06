build:
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" .