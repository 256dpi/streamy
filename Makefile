all:
	go fmt . ./max-sender ./sender
	go vet . ./max-sender ./sender
	golint . ./max-sender ./sender
